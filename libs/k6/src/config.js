export const config = {
  grafanaBaseUrl:
    __ENV.GRAFANA_BASE_URL ??
    'https://grafana.altinn.cloud',
  k6PrometheusDashboard:
    __ENV.K6_PROMETHEUS_DASHBOARD ??
    'd/ccbb2351-2ae2-462f-ae0e-f2c893ad1028/k6-prometheus',
  datasource: __ENV.DATASOURCE ?? 'k6tests-amw',
};
