import * as core from "@actions/core";
import * as github from "@actions/github";
import { DefaultAzureCredential } from "@azure/identity";
import { ReadableSpan, Span, SpanProcessor } from "@opentelemetry/sdk-trace-base";
import {
  trace,
  context,
  SpanKind
} from "@opentelemetry/api";
import { useAzureMonitor, AzureMonitorOpenTelemetryOptions, shutdownAzureMonitor } from "@azure/monitor-opentelemetry";

async function run() {
  try {
    // Retrieve inputs
    const connectionString = core.getInput("connection_string");
    const app = core.getInput("app");
    const team = core.getInput("team");
    const token = core.getInput("repo_token")
    const environment = core.getInput("environment");
    
    if (connectionString.trim() === "") {
      throw new Error("Application Insights connection string is required.");
    }

    if (app.trim() === "") {
      throw new Error("App name is required.");
    }

    if (team.trim() === "") {
      throw new Error("Team name is required.");
    }

    if (token.trim() === "") {
      throw new Error("GitHub token is required. Ensure 'repo_token' input is provided.");
    }

    // Add default attributes to spans
    class SpanEnrichingProcessor implements SpanProcessor {
      forceFlush(): Promise<void> {
        return Promise.resolve();
      }

      shutdown(): Promise<void> {
        return Promise.resolve();
      }

      onStart(_span: Span): void {}

      onEnd(span: ReadableSpan) {

        span.attributes["source"] = "gh-action-tracer";
        span.attributes["appName"] = app;
        span.attributes["team"] = team;
        span.attributes["git_hash"] = github.context.sha;
        span.attributes["environment"] = environment;

      }
    }
    
    let credential;
    try {
      credential = new DefaultAzureCredential();
    } catch (error) {
      console.warn("Azure credential not available, proceeding without it.");
    }

    const options: AzureMonitorOpenTelemetryOptions = {
      azureMonitorExporterOptions: {
        connectionString: connectionString,
        ...(credential && { credential: credential }),
      },
      spanProcessors: [new SpanEnrichingProcessor()] 
    };

    // Initialize otel with Azure Monitor
    useAzureMonitor(options);

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
    function parseDate(
      dateString: string | null | undefined,
      defaultTime: Date
    ): Date {
      if (dateString && dateString !== "null" && !isNaN(Date.parse(dateString))) {
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

    const workflowSpan = tracer.startSpan(
      `Workflow: ${workflowRun.name ?? "Unnamed Workflow"}`,
      {
        startTime: workflowStartTime,
        kind: SpanKind.INTERNAL,
        attributes: {
          repository: repo.repo,
          run_id: runId.toString(),
          workflow_name: workflowRun.name ?? undefined,
          status: workflowRun.status ?? undefined,
          run_started_at: workflowRun.run_started_at ?? undefined,
          created_at: workflowRun.created_at ?? undefined,
          updated_at: workflowRun.updated_at ?? undefined,
        },
      }
    );

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

        const jobSpan = tracer.startSpan(`Job: ${job.name ?? "Unnamed Job"}`, {
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
            // Skip steps with missing timestamps
            if (!step.started_at && !step.completed_at) {
              console.warn(`Skipping step "${step.name}" due to missing timestamps.`);
              continue;
            }

            const stepStartTime = parseDate(step.started_at, jobStartTime);
            const stepEndTime = parseDate(step.completed_at, jobEndTime);

            // Ensure end time is not before start time
            if (stepEndTime < stepStartTime) {
              console.warn(
                `Step "${step.name}" end time is before start time. Adjusting end time to match start time.`
              );
              stepEndTime.setTime(stepStartTime.getTime());
            }

            const stepSpan = tracer.startSpan(`Step: ${step.name ?? "Unnamed Step"}`, {
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
    console.log("Trace ID:", workflowSpan.spanContext().traceId);

    console.log("Trace data sent to Azure Monitor successfully.");
  } catch (error) {
    if (error instanceof Error) {
      core.setFailed(`Action failed with error: ${error.message}`);
    } else {
      core.setFailed(`Action failed with unexpected error: ${JSON.stringify(error)}`);
    }
  }
}

run().catch(async (error) => {
  console.error("An error occurred:", error);
  await shutdownAzureMonitor();
  process.exit(1);
});
