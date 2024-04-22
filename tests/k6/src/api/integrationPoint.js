import http from "k6/http";
import { generateQueryParamString } from "../apiHelpers.js";
import * as config from "../config.js";
import { stopIterationOnFail } from "../errorhandler.js";

const apimSubscriptionKey = __ENV.apimSubscriptionKey;

export function GetLevetidUtloptLast20() {
  if (!apimSubscriptionKey) {
    stopIterationOnFail(
      "Required environment variable APIM subscription key (apimSubscriptionKey) was not provided",
      false
    );
  }

  const now = new Date();

  const fromDateTime = new Date(now.getTime() - 20 * 60 * 1000).toISOString();
  const toDateTime = new Date().toISOString();

  var params = {
    "subscription-key": apimSubscriptionKey,
    status: "LEVETID_UTLOPT",
    fromDateTime: fromDateTime,
    toDateTime: toDateTime,
  };

  var endpoint =
    config.integrationPoint.status + generateQueryParamString(params);

  var response = http.get(endpoint);

  return response;
}

export function GetAvailability() {
  if (!apimSubscriptionKey) {
    stopIterationOnFail(
      "Required environment variable APIM subscription key (apimSubscriptionKey) was not provided",
      false
    );
  }

  var endpoint =
    config.integrationPoint.availability +
    "?subscription-key=" +
    apimSubscriptionKey;

  var response = http.get(endpoint);

  return response;
}
