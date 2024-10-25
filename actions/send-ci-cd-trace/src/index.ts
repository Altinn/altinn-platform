import * as core from "@actions/core";
import * as github from "@actions/github";
import { DefaultAzureCredential } from "@azure/identity";
import { NodeTracerProvider } from "@opentelemetry/sdk-trace-node";
import { AzureMonitorTraceExporter } from "@azure/monitor-opentelemetry-exporter";
import { BatchSpanProcessor } from "@opentelemetry/sdk-trace-base";
import {
  trace,
  context,
  SpanKind,
  diag,
  DiagConsoleLogger,
  DiagLogLevel,
} from "@opentelemetry/api";
import { Resource } from "@opentelemetry/resources";
import { SemanticResourceAttributes } from "@opentelemetry/semantic-conventions";

// Set the global logger and log level for debugging
diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.ALL);

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

    // Set up OpenTelemetry Tracer Provider
    const provider = new NodeTracerProvider({
      resource: new Resource({
        [SemanticResourceAttributes.SERVICE_NAME]: "gh-action-tracer",
      }),
    });

    // Set up Azure Monitor Exporter with Azure Identity
    const credential = new DefaultAzureCredential();
    const exporter = new AzureMonitorTraceExporter({
      connectionString: connectionString,
      credential: credential,
    });

    provider.addSpanProcessor(new BatchSpanProcessor(exporter));
    provider.register();

    // Get tracer
    const tracer = trace.getTracer("gh-action-tracer");

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
    function parseDate(dateString: string | null | undefined, defaultTime: Date): Date {
      if (dateString && !isNaN(Date.parse(dateString))) {
        return new Date(dateString);
      } else {
        console.warn(`Invalid or missing date string "${dateString}", using default time.`);
        return defaultTime;
      }
    }

    // Start a root span for the workflow
    const workflowStartTime = parseDate(
      workflowRun.run_started_at || workflowRun.created_at,
      new Date()
    );
    const workflowEndTime = parseDate(workflowRun.updated_at, new Date());

    const workflowSpan = tracer.startSpan(`Workflow: ${workflowRun.name ?? 'Unnamed Workflow'}`, {
      startTime: workflowStartTime,
      kind: SpanKind.INTERNAL,
      attributes: {
        app: repo.repo,
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
    await context.with(trace.setSpan(context.active(), workflowSpan), async () => {
      // Iterate over the jobs and create spans for each job
      for (const job of jobs) {
        const jobStartTime = parseDate(job.started_at, workflowStartTime);
        const jobEndTime = parseDate(job.completed_at, workflowEndTime);

        // Ensure end time is not before start time
        if (jobEndTime < jobStartTime) {
          console.warn(
            `Job "${job.name}" end time is before start time. Adjusting end time to match start time.`
          );
          jobEndTime.setTime(jobStartTime.getTime());
        }

        const jobSpan = tracer.startSpan(`Job: ${job.name ?? 'Unnamed Job'}`, {
          startTime: jobStartTime,
          kind: SpanKind.INTERNAL,
          attributes: {
            job_name: job.name ?? undefined,
            status: job.conclusion ?? undefined,
            started_at: job.started_at ?? undefined,
            completed_at: job.completed_at ?? undefined,
          },
        });

        // Iterate over the steps in each job and create spans for each step
        const steps = job.steps || [];
        await context.with(trace.setSpan(context.active(), jobSpan), async () => {
          for (const step of steps) {
            const stepStartTime = parseDate(step.started_at, jobStartTime);
            const stepEndTime = parseDate(step.completed_at, jobEndTime);

            // Ensure end time is not before start time
            if (stepEndTime < stepStartTime) {
              console.warn(
                `Step "${step.name}" end time is before start time. Adjusting end time to match start time.`
              );
              stepEndTime.setTime(stepStartTime.getTime());
            }

            const stepSpan = tracer.startSpan(`Step: ${step.name ?? 'Unnamed Step'}`, {
              startTime: stepStartTime,
              kind: SpanKind.INTERNAL,
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

    // Force flush and shutdown the provider to ensure all spans are sent
    await provider.forceFlush();
    await provider.shutdown();

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
