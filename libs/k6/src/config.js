export const config = {
  grafanaBaseUrl:
    __ENV.GRAFANA_BASE_URL ??
    'https://altinn-grafana-test-b2b8dpdkcvfuhfd3.eno.grafana.azure.com',
  k6PrometheusDashboard:
    __ENV.K6_PROMETHEUS_DASHBOARD ??
    'd/ccbb2351-2ae2-462f-ae0e-f2c893ad1028/k6-prometheus',
  datasource: __ENV.DATASOURCE ?? 'k6tests-amw',
};

export const maskinporten = {
  audience: 'https://test.maskinporten.no/',
  token: 'https://test.maskinporten.no/token',
};
