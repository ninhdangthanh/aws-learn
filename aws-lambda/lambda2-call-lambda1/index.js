const { LambdaClient, InvokeCommand } = require("@aws-sdk/client-lambda");

const client = new LambdaClient({});

exports.handler = async (event) => {
  const message = event?.message ?? "Hello from Lambda 2";
  const targetFunctionName = process.env.LAMBDA_1_FUNCTION_NAME;

  if (!targetFunctionName) {
    throw new Error("Missing environment variable: LAMBDA_1_FUNCTION_NAME");
  }

  const command = new InvokeCommand({
    FunctionName: targetFunctionName,
    InvocationType: "RequestResponse",
    Payload: Buffer.from(JSON.stringify({ message })),
  });

  const response = await client.send(command);
  const payloadText = response.Payload
    ? Buffer.from(response.Payload).toString("utf-8")
    : "";
  let lambda1Response = null;
  if (payloadText) {
    try {
      lambda1Response = JSON.parse(payloadText);
    } catch {
      lambda1Response = payloadText;
    }
  }

  return {
    statusCode: 200,
    body: JSON.stringify({
      ok: true,
      invokedFunction: targetFunctionName,
      lambda1Response,
    }),
  };
};