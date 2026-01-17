# Deployment & Testing Guide

## Prerequisites

Before deploying, ensure you have:

1. **AWS CLI** installed and configured
   ```bash
   # Install AWS CLI (if not installed)
   brew install awscli
   
   # Configure with your credentials
   aws configure
   ```

2. **AWS SAM CLI** installed
   ```bash
   brew install aws-sam-cli
   ```

3. **Docker** (for local testing)
   ```bash
   # Install Docker Desktop from https://docker.com
   ```

---

## Step 1: Build the Project

```bash
cd /Users/manishgupta/Desktop/booking-villa-backend

# Download dependencies
go mod tidy

# Build for Lambda (ARM64 architecture)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap ./cmd/main.go

# Or use Makefile
make build
```

---

## Step 2: Deploy to AWS

### Option A: Full SAM Deployment (Recommended)

This creates **everything automatically** (Lambda, API Gateway, DynamoDB):

```bash
# First-time deployment (interactive wizard)
sam deploy --guided
```

You'll be prompted for:
- **Stack Name**: `booking-villa-backend` (or your choice)
- **AWS Region**: `ap-south-1` (or your preferred region)
- **JWTSecret**: Enter a strong secret key (e.g., `my-super-secret-key-123`)
- **Confirm changes**: `Y`
- **Allow SAM to create IAM roles**: `Y`
- **Save arguments to config file**: `Y`

After first deployment, subsequent deploys are simpler:
```bash
sam deploy
```

### Option B: SAM Build + Deploy

```bash
# Build with SAM (creates .aws-sam directory)
sam build

# Deploy
sam deploy --guided
```

---

## Step 3: Manual DynamoDB Table Creation (Alternative)

If you want to create the table **manually** (without SAM), run this:

```bash
# Create the DynamoDB table
aws dynamodb create-table \
  --table-name BookingPlatformTable \
  --attribute-definitions \
    AttributeName=PK,AttributeType=S \
    AttributeName=SK,AttributeType=S \
    AttributeName=GSI1PK,AttributeType=S \
    AttributeName=GSI1SK,AttributeType=S \
  --key-schema \
    AttributeName=PK,KeyType=HASH \
    AttributeName=SK,KeyType=RANGE \
  --global-secondary-indexes \
    '[{
      "IndexName": "GSI1",
      "KeySchema": [
        {"AttributeName": "GSI1PK", "KeyType": "HASH"},
        {"AttributeName": "GSI1SK", "KeyType": "RANGE"}
      ],
      "Projection": {"ProjectionType": "ALL"}
    }]' \
  --billing-mode PAY_PER_REQUEST \
  --region ap-south-1

# Enable TTL for automatic OTP cleanup
aws dynamodb update-time-to-live \
  --table-name BookingPlatformTable \
  --time-to-live-specification "Enabled=true, AttributeName=TTL" \
  --region ap-south-1
```

---

## Step 4: Verify Deployment

### Check API Gateway Endpoint

After deployment, SAM outputs the API URL:
```
Outputs:
-----------------------------------------
Key: ApiEndpoint
Value: https://xxxxxxxxxx.execute-api.ap-south-1.amazonaws.com/prod
```

### Test Health Endpoint
```bash
# Replace with your actual API endpoint
API_URL="https://xxxxxxxxxx.execute-api.ap-south-1.amazonaws.com/prod"

curl $API_URL/health
# Expected: {"service":"booking-villa-backend","status":"healthy"}
```

---

## Step 5: Test the APIs

### 1. Send OTP
```bash
curl -X POST $API_URL/auth/send-otp \
  -H "Content-Type: application/json" \
  -d '{"phone": "9876543210"}'
```

**Response:**
```json
{
  "message": "OTP sent successfully",
  "phone": "9876543210",
  "code": "123456"  // Only in dev mode
}
```

### 2. Verify OTP (creates user automatically)
```bash
curl -X POST $API_URL/auth/verify-otp \
  -H "Content-Type: application/json" \
  -d '{
    "phone": "9876543210",
    "code": "123456",
    "name": "Admin User",
    "role": "admin"
  }'
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "phone": "9876543210",
    "name": "Admin User",
    "role": "admin",
    "status": "pending"
  },
  "isNew": true
}
```

### 3. Create a Property (with JWT)
```bash
TOKEN="your-jwt-token-here"

curl -X POST $API_URL/properties \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Beach Villa",
    "address": "123 Beach Road",
    "city": "Goa",
    "country": "India",
    "pricePerNight": 5000,
    "maxGuests": 6,
    "bedrooms": 3,
    "bathrooms": 2
  }'
```

### 4. Create Invite Code
```bash
curl -X POST $API_URL/properties/{property-id}/invite-codes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"expiresInDays": 30}'
```

### 5. Create Booking
```bash
curl -X POST $API_URL/bookings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "propertyId": "property-uuid",
    "guestName": "John Doe",
    "guestPhone": "9998887776",
    "checkIn": "2026-02-01",
    "checkOut": "2026-02-05",
    "numGuests": 4
  }'
```

### 6. Log Payment
```bash
curl -X POST $API_URL/bookings/{booking-id}/payments \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "amount": 10000,
    "method": "upi",
    "reference": "UPI-12345"
  }'
```

---

## Check DynamoDB Data

### View All Items in Table
```bash
aws dynamodb scan \
  --table-name BookingPlatformTable \
  --region ap-south-1 \
  --output json | jq '.Items'
```

### Query Users
```bash
aws dynamodb query \
  --table-name BookingPlatformTable \
  --key-condition-expression "PK = :pk" \
  --expression-attribute-values '{":pk": {"S": "USER#9876543210"}}' \
  --region ap-south-1
```

### Query Properties by Owner
```bash
aws dynamodb query \
  --table-name BookingPlatformTable \
  --index-name GSI1 \
  --key-condition-expression "GSI1PK = :gsi1pk" \
  --expression-attribute-values '{":gsi1pk": {"S": "OWNER#9876543210"}}' \
  --region ap-south-1
```

---

## Local Testing (Without AWS)

```bash
# Start local API (requires Docker)
sam local start-api

# Test locally at http://localhost:3000
curl http://localhost:3000/health
```

---

## View Lambda Logs

```bash
# Get recent logs
sam logs -n BookingFunction --stack-name booking-villa-backend --tail

# Or use AWS CLI
aws logs tail /aws/lambda/booking-villa-backend-BookingFunction --follow
```

---

## Cleanup (Delete Everything)

```bash
# Delete the CloudFormation stack (removes Lambda, API Gateway, DynamoDB)
sam delete --stack-name booking-villa-backend

# Or manually delete table
aws dynamodb delete-table --table-name BookingPlatformTable --region ap-south-1
```
