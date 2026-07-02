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
	Run(string) (api.PipelineRun, error)
	CancelRun(string, string) (api.PipelineRun, error)
	RetryRun(string, string) (api.PipelineRun, error)
	CreateProject(api.CreateProjectRequest, ...string) (api.Project, error)
	SubmitPipeline(api.SubmitPipelineRequest, ...string) (api.PipelineRun, error)
	RegisterModel(api.RegisterModelRequest, string) (api.Model, error)
	PromoteModel(string, string, string) (api.Model, error)
	DeployModel(string, int, string) (api.Model, error)
	RollbackModel(string, string) (api.Model, error)
	DeployAgent(api.DeployAgentRequest, string) (api.Agent, error)
	SetAgentTraffic(string, int, string) (api.Agent, error)
	RegisterTool(api.RegisterToolRequest, string) (api.Tool, error)
	CreateConnection(api.CreateConnectionRequest, string) (api.Connection, error)
	UpdateConnectionStatus(string, string, string, string) (api.Connection, error)
	AgentSessions(string) []api.AgentSession
	AgentTraces(string) []api.AgentTrace
	RecordTrace(api.RecordTraceRequest) (api.AgentTrace, error)
	FeatureViews() []api.FeatureView
	ApplyFeatureView(api.ApplyFeatureViewRequest, string) (api.FeatureView, error)
	ReportMaterialization(string, int, string) (api.FeatureView, error)
}
