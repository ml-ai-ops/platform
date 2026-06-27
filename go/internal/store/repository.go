package store

import "github.com/mlaiops/platform/pkg/api"

type Repository interface {
	Projects() []api.Project
	Runs() []api.PipelineRun
	Models() []api.Model
	Agents() []api.Agent
	Tools() []api.Tool
	Connections() []api.Connection
	Audit() []api.AuditEvent
	CreateProject(api.CreateProjectRequest, ...string) (api.Project, error)
	SubmitPipeline(api.SubmitPipelineRequest, ...string) (api.PipelineRun, error)
	RegisterModel(api.RegisterModelRequest, string) (api.Model, error)
	PromoteModel(string, string, string) (api.Model, error)
	DeployAgent(api.DeployAgentRequest, string) (api.Agent, error)
	SetAgentTraffic(string, int, string) (api.Agent, error)
	RegisterTool(api.RegisterToolRequest, string) (api.Tool, error)
	CreateConnection(api.CreateConnectionRequest, string) (api.Connection, error)
}
