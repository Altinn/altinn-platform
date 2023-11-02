// Baseurls for platform
export var baseUrls = {
  tt02: "tt02.altinn.no",
  prod: "altinn.no",
};

//Get values from environment
const environment = __ENV.env.toLowerCase();
export let baseUrl = baseUrls[environment];


// Integration point
export var integrationPoint = {
  status:
  "https://platform." + baseUrl + "/eformidling/api/statuses/",
  availability:
  "https://platform." + baseUrl + "/eformidling/api/manage/availability/",

}