import { group } from 'k6';

export function describe(name, fn) {
    let success = true;
  
    group(name, () => {
      try {
        const result = fn();
        success = true;
        if (result instanceof Promise) {
          // If it's a promise, wait for it to finish and handle errors
          result.then(() => {
              success = true;
          }).catch(error => {
            handleError(name, error);
          });
        }
      }
      catch (error) {
        handleError(name, error);
      }
    });
  
    return success;
  }

  function handleError(name, error) {
    if (error.name !== 'AssertionError') {
      // Goja (the JS engine used by K6) seems to clobber the stack when rethrowing exceptions
      console.error(error.stack);
      throw error;
    }
    let errmsg = `${name} failed, ${error.message}`;
    if (error.expected) {
        errmsg += ` expected:${error.expected} actual:${error.actual}`;
    }
    console.warn(errmsg);
  }