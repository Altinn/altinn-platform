var originalLog = console.log;

function customDir(obj) {
    var seen = [];

    var replacer = function(key, value) {
      if (typeof value === "object" && value !== null) {
        if (seen.indexOf(value) !== -1) {
          return "[Circular]";
        }
        seen.push(value);
      }
      return value;
    };

    var stringified = JSON.stringify(obj, replacer, 2);
    originalLog(stringified);
}

export var customConsole = {
  log: originalLog,
  dir: customDir,
  // You can manually add other console methods if needed, such as warn, error, etc.
  warn: console.warn,
  error: console.error
};
