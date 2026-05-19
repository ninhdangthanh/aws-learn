exports.handler = async (event) => {
    const triggeredTime = event?.time || new Date().toISOString();
    const message = event?.message;

    console.log(`Lambda 5 triggered at: ${triggeredTime}`);
    if (message) {
        console.log(`Event message: ${message}`);
    }

    return {
        statusCode: 200,
        body: JSON.stringify({
            ok: true,
            triggeredTime,
            message,
        }),
    };
};
