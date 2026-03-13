exports.handler = async (event) => {
  let bodyMessage;
  if (typeof event?.body === "string") {
    try {
      bodyMessage = JSON.parse(event.body)?.message;
    } catch {
      bodyMessage = undefined;
    }
  } else {
    bodyMessage = event?.body?.message;
  }

  const message = event?.message ?? bodyMessage ?? "No message provided";

  console.log("Lambda 1 received message:", message);

  return {
    statusCode: 200,
    body: JSON.stringify({
      ok: true,
      receivedMessage: message,
    }),
  };
};