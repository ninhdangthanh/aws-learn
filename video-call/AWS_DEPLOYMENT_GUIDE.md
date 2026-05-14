# 🚀 AWS ECS Deployment Guide

This guide provides step-by-step instructions for deploying the Video Call application to AWS using ECS Fargate and GitHub Actions.

---

## 1. Prerequisites
- An **AWS Account**.
- **AWS CLI** installed and configured.
- **GitHub** repository for your code.
- A **Domain Name** (optional, but recommended for production HTTPS).

---

## 2. Infrastructure Setup (AWS Console)

### A. Amazon ECR (Elastic Container Registry)
You need two repositories to store your Docker images:
1.  Go to **ECR** > **Repositories**.
2.  Create a private repository named `videocall-backend`.
3.  Create a private repository named `videocall-frontend`.
4.  Keep the URLs (e.g., `<ACCOUNT_ID>.dkr.ecr.<REGION>.amazonaws.com/videocall-backend`).

### B. CloudWatch Logs
ECS needs a place to send logs:
1.  Go to **CloudWatch** > **Log groups**.
2.  Create a log group named `/ecs/videocall`.

### C. IAM Roles
Ensure you have the `ecsTaskExecutionRole`. 
1.  Go to **IAM** > **Roles**.
2.  Check if `ecsTaskExecutionRole` exists. If not, create it.
3.  Attach the policy `AmazonECSTaskExecutionRolePolicy`.
4.  Copy the **ARN** (you'll need it for `task-definition.json`).

### D. ECS Cluster
1.  Go to **ECS** > **Clusters**.
2.  Create a new cluster named `videocall-cluster`.
3.  Choose **AWS Fargate (serverless)**.

---

## 3. Load Balancer (ALB)
Since we have a frontend, we need an ALB to route traffic:
1.  Go to **EC2** > **Load Balancers**.
2.  Create an **Application Load Balancer**.
3.  **Listeners**: Port 80 (HTTP).
4.  **Security Group**: Allow Port 80 from `0.0.0.0/0`.
5.  **Target Group**: Create a target group for the **Frontend**:
    *   Type: **IP**.
    *   Protocol: **HTTP**, Port: **80**.
    *   Health check path: `/`.

---

## 4. GitHub Actions Configuration

### A. Set Repository Secrets
In your GitHub Repo, go to **Settings** > **Secrets and variables** > **Actions** and add:
- `AWS_ACCESS_KEY_ID`: Your AWS User access key.
- `AWS_SECRET_ACCESS_KEY`: Your AWS User secret key.

### B. Update Configuration Files
1.  **`.aws/task-definition.json`**:
    *   Replace `<ACCOUNT_ID>` with your AWS Account ID.
    *   Ensure the `executionRoleArn` matches your IAM role ARN.
2.  **`.github/workflows/deploy.yml`**:
    *   Update `AWS_REGION` if you aren't using `us-east-1`.

---

## 5. Deployment

### A. Initial Task Definition
Before the CI/CD works, you must register the first task definition:
```bash
aws ecs register-task-definition --cli-input-json file://.aws/task-definition.json
```

### B. Create ECS Service
1.  In your **ECS Cluster**, go to the **Services** tab.
2.  Click **Create**.
3.  **Deployment configuration**:
    *   Family: `videocall-task`.
    *   Service name: `videocall-service`.
    *   Desired tasks: `1`.
4.  **Networking**:
    *   Select your VPC and Subnets.
    *   **Security Group**: Allow Port 80 (from ALB) and Port 8080 (optional, for direct access).
5.  **Load balancing**:
    *   Select your ALB and the Frontend target group.

### C. Trigger CI/CD
Simply commit and push your changes to the `main` branch:
```bash
git add .
git commit -m "Setup AWS Deployment"
git push origin main
```
GitHub Actions will now build the images, push them to ECR, and update the ECS service.

---

## 💾 Data Persistence (SQLite)
By default, the SQLite database (`videocall.db`) is stored inside the Fargate task. If the task restarts, data is lost.
**For production:**
1.  Create an **Amazon EFS** (Elastic File System).
2.  Update the `task-definition.json` to mount the EFS volume to `/app`.
3.  Or, refactor the backend to use **Amazon RDS** (PostgreSQL/MySQL).

## 🔒 HTTPS (SSL)
To enable HTTPS:
1.  Request a certificate in **AWS Certificate Manager (ACM)**.
2.  Update your **ALB Listener** to Port 443 (HTTPS) and attach the certificate.
3.  Update `frontend/src/config.ts` to use `https://` and `wss://`.
