package engine

import (
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/pivotal-golang/lager"
)

type implicitOutput struct {
	plan atc.GetPlan
	info exec.VersionInfo
}

//go:generate counterfeiter . BuildDelegate

type BuildDelegate interface {
	InputDelegate(lager.Logger, atc.GetPlan, event.OriginLocation) exec.GetDelegate
	ExecutionDelegate(lager.Logger, atc.TaskPlan, event.OriginLocation) exec.TaskDelegate
	OutputDelegate(lager.Logger, atc.PutPlan, event.OriginLocation) exec.PutDelegate

	Finish(lager.Logger, error, exec.Success, bool)
}

//go:generate counterfeiter . BuildDelegateFactory

type BuildDelegateFactory interface {
	Delegate(buildID int) BuildDelegate
}

type buildDelegateFactory struct {
	db EngineDB
}

func NewBuildDelegateFactory(db EngineDB) BuildDelegateFactory {
	return buildDelegateFactory{db}
}

func (factory buildDelegateFactory) Delegate(buildID int) BuildDelegate {
	return newBuildDelegate(factory.db, buildID)
}

type delegate struct {
	db EngineDB

	buildID int

	implicitOutputs map[string]implicitOutput

	lock sync.Mutex
}

func newBuildDelegate(db EngineDB, buildID int) BuildDelegate {
	return &delegate{
		db: db,

		buildID: buildID,

		implicitOutputs: make(map[string]implicitOutput),
	}
}

func (delegate *delegate) InputDelegate(logger lager.Logger, plan atc.GetPlan, location event.OriginLocation) exec.GetDelegate {
	return &inputDelegate{
		logger:   logger,
		plan:     plan,
		location: location,
		delegate: delegate,
	}
}

func (delegate *delegate) OutputDelegate(logger lager.Logger, plan atc.PutPlan, location event.OriginLocation) exec.PutDelegate {
	return &outputDelegate{
		logger:   logger,
		plan:     plan,
		location: location,
		delegate: delegate,
	}
}

func (delegate *delegate) ExecutionDelegate(logger lager.Logger, plan atc.TaskPlan, location event.OriginLocation) exec.TaskDelegate {
	return &executionDelegate{
		logger:   logger,
		plan:     plan,
		location: location,
		delegate: delegate,
	}
}

func (delegate *delegate) Finish(logger lager.Logger, err error, succeeded exec.Success, aborted bool) {
	if aborted {
		delegate.saveStatus(logger, atc.StatusAborted)

		logger.Info("aborted")
	} else if err != nil {
		delegate.saveStatus(logger, atc.StatusErrored)

		logger.Info("errored", lager.Data{"error": err.Error()})
	} else if bool(succeeded) {
		delegate.saveStatus(logger, atc.StatusSucceeded)

		implicits := logger.Session("implicit-outputs")

		for _, o := range delegate.implicitOutputs {
			delegate.saveImplicitOutput(implicits.Session(o.plan.Name), o.plan, o.info)
		}

		logger.Info("succeeded")
	} else {
		delegate.saveStatus(logger, atc.StatusFailed)

		logger.Info("failed")
	}
}

func (delegate *delegate) registerImplicitOutput(resource string, output implicitOutput) {
	delegate.lock.Lock()
	delegate.implicitOutputs[resource] = output
	delegate.lock.Unlock()
}

func (delegate *delegate) unregisterImplicitOutput(resource string) {
	delegate.lock.Lock()
	delete(delegate.implicitOutputs, resource)
	delegate.lock.Unlock()
}

func (delegate *delegate) saveInitialize(logger lager.Logger, taskConfig atc.TaskConfig, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.InitializeTask{
		TaskConfig: event.ShadowTaskConfig(taskConfig),
		Origin:     origin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (delegate *delegate) saveStart(logger lager.Logger, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.StartTask{
		Time:   time.Now().Unix(),
		Origin: origin,
	})
	if err != nil {
		logger.Error("failed-to-save-start-event", err)
	}
}

func (delegate *delegate) saveFinish(logger lager.Logger, status exec.ExitStatus, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.FinishTask{
		ExitStatus: int(status),
		Time:       time.Now().Unix(),
		Origin:     origin,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
	}
}

func (delegate *delegate) saveStatus(logger lager.Logger, status atc.BuildStatus) {
	err := delegate.db.FinishBuild(delegate.buildID, db.Status(status))
	if err != nil {
		logger.Error("failed-to-finish-build", err)
	}
}

func (delegate *delegate) saveErr(logger lager.Logger, errVal error, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.Error{
		Message: errVal.Error(),
		Origin:  origin,
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}
}

func (delegate *delegate) saveInput(logger lager.Logger, status exec.ExitStatus, plan atc.GetPlan, info *exec.VersionInfo, origin event.Origin) {
	var version atc.Version
	var metadata []atc.MetadataField

	if info != nil && plan.Pipeline != "" {
		savedVR, err := delegate.db.SaveBuildInput(delegate.buildID, db.BuildInput{
			Name:              plan.Name,
			VersionedResource: vrFromInput(plan, *info),
		})
		if err != nil {
			logger.Error("failed-to-save-input", err)
		}

		version = atc.Version(savedVR.Version)
		metadata = dbMetadataToATCMetadata(savedVR.Metadata)
	}

	ev := event.FinishGet{
		Origin: origin,
		Plan: event.GetPlan{
			Name:     plan.Name,
			Resource: plan.Resource,
			Type:     plan.Type,
			Version:  plan.Version,
		},
		ExitStatus:      int(status),
		FetchedVersion:  version,
		FetchedMetadata: metadata,
	}

	err := delegate.db.SaveBuildEvent(delegate.buildID, ev)
	if err != nil {
		logger.Error("failed-to-save-input-event", err)
	}
}

func (delegate *delegate) saveOutput(logger lager.Logger, status exec.ExitStatus, plan atc.PutPlan, info *exec.VersionInfo, origin event.Origin) {
	var version atc.Version
	var metadata []atc.MetadataField

	if info != nil {
		version = info.Version
		metadata = info.Metadata
	}

	ev := event.FinishPut{
		Origin: origin,
		Plan: event.PutPlan{
			Name:     plan.Name,
			Resource: plan.Resource,
			Type:     plan.Type,
		},
		ExitStatus:      int(status),
		CreatedVersion:  version,
		CreatedMetadata: metadata,
	}

	err := delegate.db.SaveBuildEvent(delegate.buildID, ev)
	if err != nil {
		logger.Error("failed-to-save-output-event", err)
	}

	if info != nil && plan.Pipeline != "" {
		_, err = delegate.db.SaveBuildOutput(delegate.buildID, vrFromOutput(plan.Pipeline, ev), true)
		if err != nil {
			logger.Error("failed-to-save-output", err)
		}
	}
}

func (delegate *delegate) saveImplicitOutput(logger lager.Logger, plan atc.GetPlan, info exec.VersionInfo) {
	if plan.Pipeline == "" {
		return
	}

	metadata := make([]db.MetadataField, len(info.Metadata))
	for i, md := range info.Metadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	_, err := delegate.db.SaveBuildOutput(delegate.buildID, db.VersionedResource{
		PipelineName: plan.Pipeline,
		Resource:     plan.Resource,
		Type:         plan.Type,
		Version:      db.Version(info.Version),
		Metadata:     metadata,
	}, false)
	if err != nil {
		logger.Error("failed-to-save", err)
		return
	}

	logger.Info("saved", lager.Data{"resource": plan.Resource})
}

func (delegate *delegate) eventWriter(origin event.Origin) io.Writer {
	return &dbEventWriter{
		db:      delegate.db,
		buildID: delegate.buildID,
		origin:  origin,
	}
}

type inputDelegate struct {
	logger lager.Logger

	plan     atc.GetPlan
	location event.OriginLocation
	delegate *delegate
}

func (input *inputDelegate) Completed(status exec.ExitStatus, info *exec.VersionInfo) {
	input.delegate.saveInput(input.logger, status, input.plan, info, event.Origin{
		Type:     event.OriginTypeGet,
		Name:     input.plan.Name,
		Location: input.location,
	})

	if info != nil {
		input.delegate.registerImplicitOutput(input.plan.Resource, implicitOutput{input.plan, *info})
	}

	input.logger.Info("finished", lager.Data{"version-info": info})
}

func (input *inputDelegate) Failed(err error) {
	input.delegate.saveErr(input.logger, err, event.Origin{
		Type:     event.OriginTypeGet,
		Name:     input.plan.Name,
		Location: input.location,
	})

	input.logger.Info("errored", lager.Data{"error": err.Error()})
}

func (input *inputDelegate) Stdout() io.Writer {
	return input.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypeGet,
		Name:     input.plan.Name,
		Source:   event.OriginSourceStdout,
		Location: input.location,
	})
}

func (input *inputDelegate) Stderr() io.Writer {
	return input.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypeGet,
		Name:     input.plan.Name,
		Source:   event.OriginSourceStderr,
		Location: input.location,
	})
}

type outputDelegate struct {
	logger lager.Logger

	plan     atc.PutPlan
	location event.OriginLocation

	delegate *delegate
	hook     string
}

func (output *outputDelegate) Completed(status exec.ExitStatus, info *exec.VersionInfo) {
	output.delegate.unregisterImplicitOutput(output.plan.Resource)
	output.delegate.saveOutput(output.logger, status, output.plan, info, event.Origin{
		Type:     event.OriginTypePut,
		Name:     output.plan.Name,
		Location: output.location,
	})

	output.logger.Info("finished", lager.Data{"version-info": info})
}

func (output *outputDelegate) Failed(err error) {
	output.delegate.saveErr(output.logger, err, event.Origin{
		Type:     event.OriginTypePut,
		Name:     output.plan.Name,
		Location: output.location,
	})

	output.logger.Info("errored", lager.Data{"error": err.Error()})
}

func (output *outputDelegate) Stdout() io.Writer {
	return output.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypePut,
		Name:     output.plan.Name,
		Source:   event.OriginSourceStdout,
		Location: output.location,
	})
}

func (output *outputDelegate) Stderr() io.Writer {
	return output.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypePut,
		Name:     output.plan.Name,
		Source:   event.OriginSourceStderr,
		Location: output.location,
	})
}

type executionDelegate struct {
	logger lager.Logger

	plan     atc.TaskPlan
	location event.OriginLocation

	delegate *delegate

	hook string
}

func (execution *executionDelegate) Initializing(config atc.TaskConfig) {
	execution.delegate.saveInitialize(execution.logger, config, event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Location: execution.location,
	})
}

func (execution *executionDelegate) Started() {
	execution.delegate.saveStart(execution.logger, event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Location: execution.location,
	})

	execution.logger.Info("started")
}

func (execution *executionDelegate) Finished(status exec.ExitStatus) {
	execution.delegate.saveFinish(execution.logger, status, event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Location: execution.location,
	})
}

func (execution *executionDelegate) Failed(err error) {
	execution.delegate.saveErr(execution.logger, err, event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Location: execution.location,
	})
	execution.logger.Info("errored", lager.Data{"error": err.Error()})
}

func (execution *executionDelegate) Stdout() io.Writer {
	return execution.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Source:   event.OriginSourceStdout,
		Location: execution.location,
	})
}

func (execution *executionDelegate) Stderr() io.Writer {
	return execution.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Source:   event.OriginSourceStderr,
		Location: execution.location,
	})
}

type dbEventWriter struct {
	buildID int
	db      EngineDB

	origin event.Origin

	dangling []byte
}

func (writer *dbEventWriter) Write(data []byte) (int, error) {
	text := append(writer.dangling, data...)

	checkEncoding, _ := utf8.DecodeLastRune(text)
	if checkEncoding == utf8.RuneError {
		writer.dangling = text
		return len(data), nil
	}

	writer.dangling = nil

	writer.db.SaveBuildEvent(writer.buildID, event.Log{
		Payload: string(text),
		Origin:  writer.origin,
	})

	return len(data), nil
}

func vrFromInput(plan atc.GetPlan, fetchedInfo exec.VersionInfo) db.VersionedResource {
	return db.VersionedResource{
		Resource:     plan.Resource,
		PipelineName: plan.Pipeline,
		Type:         plan.Type,
		Version:      db.Version(fetchedInfo.Version),
		Metadata:     atcMetadataToDBMetadata(fetchedInfo.Metadata),
	}
}

func vrFromOutput(pipelineName string, putted event.FinishPut) db.VersionedResource {
	return db.VersionedResource{
		Resource:     putted.Plan.Resource,
		PipelineName: pipelineName,
		Type:         putted.Plan.Type,
		Version:      db.Version(putted.CreatedVersion),
		Metadata:     atcMetadataToDBMetadata(putted.CreatedMetadata),
	}
}

func dbMetadataToATCMetadata(dbm []db.MetadataField) []atc.MetadataField {
	metadata := make([]atc.MetadataField, len(dbm))
	for i, md := range dbm {
		metadata[i] = atc.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return metadata
}

func atcMetadataToDBMetadata(atcm []atc.MetadataField) []db.MetadataField {
	metadata := make([]db.MetadataField, len(atcm))
	for i, md := range atcm {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return metadata
}
