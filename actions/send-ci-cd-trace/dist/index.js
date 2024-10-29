"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (k !== "default" && Object.prototype.hasOwnProperty.call(mod, k)) __createBinding(result, mod, k);
    __setModuleDefault(result, mod);
    return result;
};
Object.defineProperty(exports, "__esModule", { value: true });
const core = __importStar(require("@actions/core"));
const github = __importStar(require("@actions/github"));
const identity_1 = require("@azure/identity");
const api_1 = require("@opentelemetry/api");
const monitor_opentelemetry_1 = require("@azure/monitor-opentelemetry");
async function run() {
    try {
        // Retrieve inputs
        const connectionString = core.getInput("connection_string");
        const app = core.getInput("app");
        const team = core.getInput("team");
        const token = core.getInput("repo_token");
        if (!connectionString) {
            throw new Error("Application Insights connection string is required.");
        }
        if (!app) {
            throw new Error("App name is required.");
        }
        if (!team) {
            throw new Error("Team name is required.");
        }
        if (!token) {
            throw new Error("GitHub token is required. Ensure 'repo_token' input is provided.");
        }
        // Add default attributes to spans
        class SpanEnrichingProcessor {
            forceFlush() {
                return Promise.resolve();
            }
            shutdown() {
                return Promise.resolve();
            }
            onStart(_span) { }
            onEnd(span) {
                span.attributes["service"] = "gh-action-tracer";
                span.attributes["app"] = app;
                span.attributes["team"] = team;
                span.attributes["git_hash"] = github.context.sha;
            }
        }
        const credential = new identity_1.DefaultAzureCredential();
        const options = {
            azureMonitorExporterOptions: {
                connectionString: connectionString,
                credential: credential,
            },
            spanProcessors: [new SpanEnrichingProcessor()]
        };
        // Initialize otel with Azure Monitor
        (0, monitor_opentelemetry_1.useAzureMonitor)(options);
        // Get tracer
        const tracer = api_1.trace.getTracer("gh-action-tracer");
        // Get GitHub context
        const { repo, runId } = github.context;
        const octokit = github.getOctokit(token);
        // Fetch workflow run details from GitHub API
        const getWorkflowRunResponse = await octokit.rest.actions.getWorkflowRun({
            owner: repo.owner,
            repo: repo.repo,
            run_id: runId,
        });
        const workflowRun = getWorkflowRunResponse.data;
        // Fetch jobs related to this workflow run
        const jobsResponse = await octokit.rest.actions.listJobsForWorkflowRun({
            owner: repo.owner,
            repo: repo.repo,
            run_id: runId,
        });
        const jobs = jobsResponse.data.jobs;
        // Function to parse and validate date strings
        function parseDate(dateString, defaultTime) {
            if (dateString && dateString !== "null" && !isNaN(Date.parse(dateString))) {
                return new Date(dateString);
            }
            else {
                console.warn(`Invalid or missing date string "${dateString}", using default time.`);
                return defaultTime;
            }
        }
        // Start a root span for the workflow
        const workflowStartTime = parseDate(workflowRun.run_started_at || workflowRun.created_at, new Date());
        const workflowEndTime = parseDate(workflowRun.updated_at, new Date());
        const workflowSpan = tracer.startSpan(`Workflow: ${workflowRun.name ?? "Unnamed Workflow"}`, {
            startTime: workflowStartTime,
            kind: api_1.SpanKind.INTERNAL,
            attributes: {
                repository: repo.repo,
                run_id: runId.toString(),
                workflow_name: workflowRun.name ?? undefined,
                status: workflowRun.status ?? undefined,
                run_started_at: workflowRun.run_started_at ?? undefined,
                created_at: workflowRun.created_at ?? undefined,
                updated_at: workflowRun.updated_at ?? undefined,
            },
        });
        // Use the context to ensure child spans are associated with the root span
        await api_1.context.with(api_1.trace.setSpan(api_1.context.active(), workflowSpan), async () => {
            // Iterate over the jobs and create spans for each job
            for (const job of jobs) {
                const jobStartTime = parseDate(job.started_at, workflowStartTime);
                const jobEndTime = parseDate(job.completed_at, workflowEndTime);
                // Ensure end time is not before start time
                if (jobEndTime < jobStartTime) {
                    console.warn(`Job "${job.name}" end time is before start time. Adjusting end time to match start time.`);
                    jobEndTime.setTime(jobStartTime.getTime());
                }
                const jobSpan = tracer.startSpan(`Job: ${job.name ?? "Unnamed Job"}`, {
                    startTime: jobStartTime,
                    kind: api_1.SpanKind.INTERNAL,
                    attributes: {
                        job_name: job.name ?? undefined,
                        status: job.conclusion ?? undefined,
                        started_at: job.started_at ?? undefined,
                        completed_at: job.completed_at ?? undefined,
                    },
                });
                // Iterate over the steps in each job and create spans for each step
                const steps = job.steps || [];
                await api_1.context.with(api_1.trace.setSpan(api_1.context.active(), jobSpan), async () => {
                    for (const step of steps) {
                        // Skip steps with missing timestamps
                        if (!step.started_at && !step.completed_at) {
                            console.warn(`Skipping step "${step.name}" due to missing timestamps.`);
                            continue;
                        }
                        const stepStartTime = parseDate(step.started_at, jobStartTime);
                        const stepEndTime = parseDate(step.completed_at, jobEndTime);
                        // Ensure end time is not before start time
                        if (stepEndTime < stepStartTime) {
                            console.warn(`Step "${step.name}" end time is before start time. Adjusting end time to match start time.`);
                            stepEndTime.setTime(stepStartTime.getTime());
                        }
                        const stepSpan = tracer.startSpan(`Step: ${step.name ?? "Unnamed Step"}`, {
                            startTime: stepStartTime,
                            kind: api_1.SpanKind.INTERNAL,
                            attributes: {
                                step_name: step.name ?? undefined,
                                status: step.conclusion ?? undefined,
                                started_at: step.started_at ?? undefined,
                                completed_at: step.completed_at ?? undefined,
                            },
                        });
                        stepSpan.end(stepEndTime);
                    }
                });
                jobSpan.end(jobEndTime);
            }
        });
        // End the workflow span
        workflowSpan.end(workflowEndTime);
        console.log("Trace data sent to Azure Monitor successfully.");
    }
    catch (error) {
        if (error instanceof Error) {
            core.setFailed(`Action failed with error: ${error.message}`);
        }
        else {
            core.setFailed(`Action failed with unexpected error: ${JSON.stringify(error)}`);
        }
    }
}
run().catch(async (error) => {
    console.error("An error occurred:", error);
    await (0, monitor_opentelemetry_1.shutdownAzureMonitor)();
    process.exit(1);
});
