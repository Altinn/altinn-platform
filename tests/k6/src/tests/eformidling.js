/*
    Test script of platform notifications api with org token
    Command:
    docker-compose run k6 run /src/tests/eformidling.js `
    -e env=*** `
    -e subscriptionKey=*** `
*/
import { check } from "k6";
import { addErrorCount, stopIterationOnFail } from "../errorhandler.js";
import * as integrationPointApi from "../api/integrationPoint.js";

export const options = {
  thresholds: {
    errors: ["count<1"],
  },
};

// 01 - Check availability
function TC01_CheckAvailability(){
    var response, success;

  response = integrationPointApi.GetAvailability();
  success = check(response, {
    "GET integration point availability. Status is 200 OK": (r) => r.status === 200,
    "GET integration point availability. Response in 'UP'": (r) => r.body === 'UP',

  });

  addErrorCount(success);
  if (!success) {
    stopIterationOnFail(success);
  }
}

// 02 - GET levetid utløpt instances
function TC02_GetLevetidUtloptInstances() {
  var response, success;

  response = integrationPointApi.GetLevetidUtloptLast20();
  success = check(response, {
    "GET levetid utløpt instances. Status is 200 OK": (r) => r.status === 200,
  });

  addErrorCount(success);
  if (!success) {
    stopIterationOnFail(success);
  }

  success = check(JSON.parse(response.body), {
    "GET levetid utløpt instances. No failed shipments": (object) =>
    object.totalElements === 0
  });
  addErrorCount(success);
}


/*
 * 01 - Check availability of integration point
 * 02 - GET levetid utløpt instances
 */
export default function () {
  try {
    TC01_CheckAvailability();
    TC02_GetLevetidUtloptInstances();
  } catch (error) {
    addErrorCount(false);
    throw error;
  }
}
