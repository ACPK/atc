package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func VersionedResource(vr db.VersionedResource) atc.VersionedResource {
	return atc.VersionedResource{
		Resource: vr.Resource,
		Version:  atc.Version(vr.Version),
	}
}

func SavedVersionedResource(svr db.SavedVersionedResource) atc.VersionedResource {
	var metadata []atc.MetadataField

	for _, v := range svr.Metadata {
		metadata = append(metadata, atc.MetadataField{
			Name:  v.Name,
			Value: v.Value,
		})
	}

	return atc.VersionedResource{
		ID:           svr.ID,
		PipelineName: svr.PipelineName,
		Resource:     svr.Resource,
		Type:         svr.Type,
		Version:      atc.Version(svr.Version),
		Metadata:     metadata,
	}
}
