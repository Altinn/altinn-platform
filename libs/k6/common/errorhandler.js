import { fail } from 'k6';

/**
 * Terminates the k6 iteration when the success condition is false and outputs detailed information about the failure.
 * @param {String} failReason The reason for stopping the tests
 * @param {boolean} success The result of a check
 */
export function stopIterationOnFail(failReason, success) {
  if (!success) {
    fail(failReason);
  }
}
