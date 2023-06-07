# AWS Permissions

When deploying DEEP to use a S3 backend you will need to provide the appropriate IAM role that has access to the bucket
that you want to use. Below is the AWS permissions schema that should be used for the IAM role.

## IAM Role

```json
{
    "Version": "2023-06-07",
    "Statement": [
        {
            "Sid": "DeepS3Access",
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:GetObject",
                "s3:ListBucket",
                "s3:DeleteObject",
                "s3:GetObjectTagging",
                "s3:PutObjectTagging"
            ],
            "Resource": [
                "arn:aws:s3:::<name-of-bucket>/*",
                "arn:aws:s3:::<name-of-bucket>"
            ]
        }
    ]
}
```
