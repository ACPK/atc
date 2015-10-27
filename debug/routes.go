package debug

import "github.com/tedsuo/rata"

const (
	GetVersionsDB = "GetVersionsDB"
)

var Routes = rata.Routes{
	{Path: "/debug/versions-db-dump.json", Method: "GET", Name: GetVersionsDB},
}
