# AWS EKS

This example is for use with AWK EKS directly using a deployment file in place of helm.

In this example we are deploying as a single binary only, we are also not deploying grafana, or any test apps.

## Configure

Edit the deployment.yaml file to add the AWS Access and Secret keys.

## Deploying

Configure kubectl to use the EKS cluster.

```bash
aws eks --region <AWS-Region> update-kubeconfig --name <Cluster-Name> --profile <AWS-CLI-Profile-Name>
```

Now deploy the pod to EKS.

```bash
kubectl apply -f deployment.yml
```

Note: As this is deploying to AWS there could be increased costs at AWS.

## Clean up

To remove the example run:

```bash
kubectl delete namespace deep
```

Note: This will only clean up the EKS cluster you will have to delete the S3 data yourself.
