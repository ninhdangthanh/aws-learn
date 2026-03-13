exports.handler = async (event) => {
  const records = event?.Records ?? [];

  if (records.length === 0) {
    console.log("Lambda 3: no S3 records found in event");
    return { statusCode: 200, body: "No records" };
  }

  const fileNames = records.map((record) => {
    const bucketName = record.s3?.bucket?.name;
    const fileName = decodeURIComponent(
      (record.s3?.object?.key ?? "").replace(/\+/g, " ")
    );
    console.log(`Lambda 3: file uploaded — bucket: ${bucketName}, file: ${fileName}`);
    return { bucketName, fileName };
  });

  return {
    statusCode: 200,
    body: JSON.stringify({ ok: true, files: fileNames }),
  };
};
