import * as core from "@actions/core";
import * as github from "@actions/github";
import { DefaultAzureCredential } from "@azure/identity";
import { NodeTracerProvider } from "@opentelemetry/sdk-trace-node";
import { AzureMonitorTraceExporter } from "@azure/monitor-opentelemetry-exporter";
import { BatchSpanProcessor } from "@opentelemetry/sdk-trace-base";
import { trace, context, SpanKind } from "@opentelemetry/api";

async function run() {
  try {
    // Retrieve inputs
    const connectionString = core.getInput("connection_string");
    const token = core.getInput("repo_token");

    if (!connectionString) {
      throw new Error("Application Insights connection string is required.");
    }

    if (!token) {
      throw new Error("GitHub token is required.");
    }

    // Set up Azure Monitor Exporter with Azure Identity
    const credential = new DefaultAzureCredential();
    const exporter = new AzureMonitorTraceExporter({
      connectionString: connectionString,
      credential: credential,
    });

    // Set up OpenTelemetry Tracer Provider
    const provider = new NodeTracerProvider();
    provider.addSpanProcessor(new BatchSpanProcessor(exporter));
    provider.register();

    // Get tracer
    const tracer = trace.getTracer("github-action-tracer");

    // Get GitHub context
    const { repo, runId, job } = github.context;
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

    // Start a root span for the workflow
    const workflowStartTime = new Date(workflowRun.run_started_at || workflowRun.created_at || 0);
    const workflowSpan = tracer.startSpan(`Workflow: ${workflowRun.name}`, {
      startTime: workflowStartTime,
      kind: SpanKind.INTERNAL,
      attributes: {
        'app': repo.repo,
        'repository': repo.repo,
        'run_id': runId,
        'workflow_name': workflowRun.name ?? undefined,
        'status': workflowRun.status ?? undefined,
        'run_started_at': workflowRun.run_started_at,
        'created_at': workflowRun.created_at,
        'updated_at': workflowRun.updated_at,
      },
    });

    // Use the context to ensure child spans are associated with the root span
    await context.with(trace.setSpan(context.active(), workflowSpan), async () => {
      // Iterate over the jobs and create spans for each job
      for (const job of jobs) {
        const jobStartTime = job.started_at ? new Date(job.started_at) : new Date(0);
        const jobEndTime = job.completed_at ? new Date(job.completed_at) : new Date();
        const jobDurationMs = jobEndTime.getTime() - jobStartTime.getTime();

        const jobSpan = tracer.startSpan(`Job: ${job.name}`, {
          startTime: jobStartTime,
          kind: SpanKind.INTERNAL,
          attributes: {
            'job_name': job.name,
            'status': job.conclusion ?? undefined,
            'started_at': job.started_at,
            'completed_at': job.completed_at ?? undefined,
            'duration_ms': jobDurationMs,
          },
        });

        // Iterate over the steps in each job and create spans for each step
        const steps = job.steps || [];
        await context.with(trace.setSpan(context.active(), jobSpan), async () => {
          for (const step of steps) {
            const stepStartTime = step.started_at ? new Date(step.started_at) : new Date(0);
            const stepEndTime = step.completed_at ? new Date(step.completed_at) : new Date();
            const stepDurationMs = stepEndTime.getTime() - stepStartTime.getTime();

            const stepSpan = tracer.startSpan(`Step: ${step.name}`, {
              startTime: stepStartTime,
              kind: SpanKind.INTERNAL,
              attributes: {
                'step_name': step.name,
                'status': step.conclusion ?? undefined,
                'started_at': step.started_at ?? undefined,
                'completed_at': step.completed_at ?? undefined,
                'duration_ms': stepDurationMs,
              },
            });

            stepSpan.end(stepEndTime);
          }
        });

        jobSpan.end(jobEndTime);
      }
    });

    // End the workflow span
    const workflowEndTime = new Date(workflowRun.updated_at || Date.now());
    workflowSpan.end(workflowEndTime);

    console.log("Trace data sent to Azure Monitor successfully.");
  } catch (error) {
    if (error instanceof Error) {
      core.setFailed(`Action failed with error: ${error.message}`);
    } else {
      core.setFailed(`Action failed with unexpected error: ${JSON.stringify(error)}`);
    }
  }
}

run();
