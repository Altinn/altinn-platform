import http from 'k6/http';

import { createDefaultPayload } from './payload';
import * as config from '../config';

function performanceMetrics(data) {
  const numberOfRequests = data.metrics.http_reqs.values['count'];
  const maxThroughput = data.metrics.http_reqs.values['rate'].toFixed(2);
  const avgReqDuration =
    data.metrics.http_req_duration.values['avg'].toFixed(2);
  const p95ReqDuration =
    data.metrics.http_req_duration.values['p(95)'].toFixed(2);
  const p99ReqDuration =
    data.metrics.http_req_duration.values['p(99)'].toFixed(2);

  return `*Number of Requests:* ${numberOfRequests} reqs
*Max Throughput:* ${maxThroughput} reqs/s
*Average Response Time:* ${avgReqDuration} ms
*p(95) Response Time:* ${p95ReqDuration} ms
*p(99) Response Time:* ${p99ReqDuration} ms`;
}

function checkMetrics(data) {
  let numberOfChecks = data.root_group.checks.length;
  let checksString = '';
  let checks = data.root_group.checks;

  for (let i = 0; i < numberOfChecks; i++) {
    let passingChecks = checks[i].passes;
    let totalChecks = checks[i].passes + checks[i].fails;
    let successRate = (passingChecks / totalChecks) * 100;
    checksString += `*Check*: '${checks[i].name}' had a Success Rate of: ${successRate}%\n`;
  }
  return checksString;
}

function buildPayload(data, reportType) {
  var payload = createDefaultPayload();
  let sectionBlocks = payload.attachments.find(
    (attachments) => attachments.blocks[1].type === 'section',
  );

  switch (reportType) {
    case 'performance':
      sectionBlocks.blocks[1].text.text = performanceMetrics(data);
      break;
    case 'checks':
      sectionBlocks.blocks[1].text.text = checkMetrics(data);
      break;
    default:
      sectionBlocks.blocks[1].text.text = performanceMetrics(data);
  }

  sectionBlocks.blocks[3].elements[0].url = `${config.grafanaBaseUrl}/\
${config.k6PrometheusDashboard}\
?orgId=1\
&var-DS_PROMETHEUS=${config.datasource}\
&var-namespace=${__ENV.NAMESPACE}\
&var-testid=${__ENV.TESTID}\
&from=${__ENV.MANIFEST_GENERATION_TIMESTAMP}\
&to=${new Date().getTime()}`;

  return payload;
}

function buildHeaders() {
  return {
    headers: {
      Authorization: 'Bearer ' + __ENV.SLACK_TOKEN,
      'Content-type': 'application/json',
    },
  };
}

function postSlackMessage(data, reportType = 'performance') {
  if (!__ENV.SLACK_WEBHOOK_URL) {
    console.error('SLACK_WEBHOOK_URL environment variable is not defined');
    return;
  }
  if (!__ENV.SLACK_TOKEN) {
    console.error('SLACK_TOKEN environment variable is not defined');
    return;
  }

  const headers = buildHeaders();
  var payload = buildPayload(data, reportType);
  const body = JSON.stringify(payload);

  try {
    const slackRes = http.post(__ENV.SLACK_WEBHOOK_URL, body, headers);
    if (slackRes.status != 200) {
      console.error('Could not send summary, got status ' + slackRes.status);
    }
  } catch (error) {
    console.error('Error sending Slack message:', error);
  }
}

exports.postSlackMessage = postSlackMessage;
