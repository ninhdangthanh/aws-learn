/**
 * Local test runner for Lambda 1 and Lambda 2.
 * - Lambda 1 is invoked directly.
 * - Lambda 2 is invoked with a mock LambdaClient that calls Lambda 1 locally.
 */

const lambda1 = require("./lambda1-receive-message/index");
const lambda3 = require("./lambda3-s3-trigger/index");

// ---------- helpers ----------

function mockLambdaClientFor(lambda1Handler) {
  return {
    send: async (command) => {
      const payload = JSON.parse(Buffer.from(command.input.Payload).toString("utf-8"));
      const result = await lambda1Handler(payload);
      return {
        StatusCode: 200,
        Payload: Buffer.from(JSON.stringify(result)),
      };
    },
  };
}

// ---------- test Lambda 1 directly ----------

async function testLambda1() {
  console.log("\n======= Lambda 1 direct test =======");
  const event = { message: "Hello from local test" };
  const result = await lambda1.handler(event);
  console.log("Lambda 1 result:", JSON.stringify(result, null, 2));
}

// ---------- test Lambda 2 -> Lambda 1 ----------

async function testLambda2() {
  console.log("\n======= Lambda 2 → Lambda 1 test =======");

  // Temporarily patch @aws-sdk/client-lambda with a local mock
  const Module = require("module");
  const originalLoad = Module._load;
  Module._load = function (request, ...args) {
    if (request === "@aws-sdk/client-lambda") {
      return {
        LambdaClient: class {
          constructor() {}
          send(command) {
            return mockLambdaClientFor(lambda1.handler).send(command);
          }
        },
        InvokeCommand: class {
          constructor(input) {
            this.input = input;
          }
        },
      };
    }
    return originalLoad(request, ...args);
  };

  // Set required env var
  process.env.LAMBDA_1_FUNCTION_NAME = "test1-local";

  // Clear cached lambda2 module so it picks up the mock
  delete require.cache[require.resolve("./lambda2-call-lambda1/index")];
  const lambda2 = require("./lambda2-call-lambda1/index");

  const event = { message: "Hello from Lambda 2 local test" };
  const result = await lambda2.handler(event);
  console.log("Lambda 2 result:", JSON.stringify(result, null, 2));

  // Restore original loader
  Module._load = originalLoad;
}

// ---------- test Lambda 3 (S3 trigger) ----------

async function testLambda3() {
  console.log("\n======= Lambda 3 S3 trigger test =======");

  // Simulate the S3 event payload AWS sends
  const event = {
    Records: [
      {
        eventName: "ObjectCreated:Put",
        s3: {
          bucket: { name: "my-test-bucket" },
          object: { key: "uploads%2Freport-2026.pdf", size: 204800 },
        },
      },
      {
        eventName: "ObjectCreated:Put",
        s3: {
          bucket: { name: "my-test-bucket" },
          object: { key: "images%2Fphoto+01.png", size: 10240 },
        },
      },
    ],
  };

  const result = await lambda3.handler(event);
  console.log("Lambda 3 result:", JSON.stringify(result, null, 2));
}

// ---------- run ----------

(async () => {
  try {
    await testLambda1();
    await testLambda2();
    await testLambda3();
    console.log("\nAll local tests passed.");
  } catch (err) {
    console.error("\nTest failed:", err.message);
    process.exit(1);
  }
})();
