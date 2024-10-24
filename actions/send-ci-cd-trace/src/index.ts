import * as core from "@actions/core";
import * as github from "@actions/github";
import * as appInsights from "applicationinsights";
import { DefaultAzureCredential } from "@azure/identity";

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

    // Set up Azure Application Insights with Azure Identity
    const credential = new DefaultAzureCredential();
    appInsights
      .setup(connectionString)
      .setAutoCollectRequests(true)
      .setAutoCollectPerformance(true, false)
      .setAutoCollectExceptions(true)
      .setAutoCollectDependencies(true)
      .setAutoCollectConsole(true, false)
      .setAutoCollectPreAggregatedMetrics(true)
      .setSendLiveMetrics(false)
      .setInternalLogging(false, true)
      .enableWebInstrumentation(false)
      .start();

    const client = appInsights.defaultClient;
    client.config.aadTokenCredential = credential;

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
    
    // Check if the current job is the last job in the workflow
    const isLastJob = jobs[jobs.length - 1]?.name === job;

    // Collect workflow metadata
    const workflowMetadata = {
      app: repo.repo,
      repository: repo.repo,
      run_id: runId,
      workflow_name: workflowRun.name,
      status: workflowRun.status,
      run_started_at: workflowRun.run_started_at,
      created_at: workflowRun.created_at,
      updated_at: workflowRun.updated_at,
    };

    // Track the workflow start trace
    const workflowStartTime = new Date(workflowRun.run_started_at || workflowRun.created_at || 0);
    client.trackEvent({
      name: `Workflow: ${workflowRun.name}`,
      properties: {
        ...workflowMetadata,
        startTime: workflowStartTime.toISOString(),
      },
    });

    // Iterate over the jobs and track each job's information
    for (const job of jobs) {
      // Skip the job if it is not the last one
      if (!isLastJob) {
        continue;
      }
      const jobMetadata = {
        job_name: job.name,
        status: job.conclusion,
        started_at: job.started_at,
        completed_at: job.completed_at,
      };

      const jobStartTime = job.started_at ? new Date(job.started_at) : new Date(0);
      const jobEndTime = job.completed_at ? new Date(job.completed_at) : new Date(0);
      const jobDurationMs = jobEndTime.getTime() - jobStartTime.getTime();

      client.trackEvent({
        name: `Job: ${job.name}`,
        properties: {
          ...jobMetadata,
          duration_ms: jobDurationMs,
          startTime: jobStartTime.toISOString(),
          endTime: jobEndTime.toISOString(),
        },
      });


      // Iterate over the steps in each job and track step information
      if (isLastJob) {
        const steps = job.steps || [];
        for (const step of steps) {
          // Skip the step if it is not the last step in the workflow
          if (job !== jobs[jobs.length - 1] || step !== steps[steps.length - 1]) {
            continue;
          }
        const stepMetadata = {
          step_name: step.name,
          status: step.conclusion,
          started_at: step.started_at,
          completed_at: step.completed_at,
        };

        const stepStartTime = step.started_at ? new Date(step.started_at) : new Date(0);
        const stepEndTime = step.completed_at ? new Date(step.completed_at) : new Date(0);
        const stepDurationMs = stepEndTime.getTime() - stepStartTime.getTime();

        client.trackEvent({
          name: `Step: ${step.name}`,
          properties: {
            ...stepMetadata,
            duration_ms: stepDurationMs,
            startTime: stepStartTime.toISOString(),
            endTime: stepEndTime ? stepEndTime.toISOString() : "",
          },
        });
      }
    }
  }
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
