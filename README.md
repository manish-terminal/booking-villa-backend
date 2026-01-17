# Booking Villa Backend

A production-ready Go backend for a Hotel/Villa Booking Platform running on AWS Lambda with SAM.

## Features

- **Phone-based Authentication**: OTP verification with 5-minute expiry
- **Optional Password Login**: Users can set passwords for alternative login
- **Role-Based Access Control**: Admin, Owner, and Agent roles
- **Property Management**: Create and manage properties with invite codes
- **Booking System**: Full booking lifecycle with availability checking
- **Offline Payment Tracking**: Log payments with status calculation (pending/due/completed)
- **DynamoDB Single-Table Design**: Optimized for serverless workloads

## Tech Stack

- **Runtime**: Go 1.21+
- **Cloud**: AWS Lambda, API Gateway, DynamoDB
- **IaC**: AWS SAM
- **Auth**: JWT (HS256)

## Project Structure

```
booking-villa-backend/
├── cmd/
│   └── main.go              # Lambda entry point and router
├── internal/
│   ├── auth/
│   │   ├── handler.go       # Auth HTTP handlers
│   │   ├── otp.go           # OTP generation and verification
│   │   └── service.go       # Auth business logic
│   ├── bookings/
│   │   ├── handler.go       # Booking HTTP handlers
│   │   └── service.go       # Booking operations
│   ├── db/
│   │   └── dynamo.go        # DynamoDB client and helpers
│   ├── middleware/
│   │   ├── auth.go          # JWT authentication middleware
│   │   └── rbac.go          # Role-based access control
│   ├── payments/
│   │   ├── handler.go       # Payment HTTP handlers
│   │   └── service.go       # Payment tracking
│   ├── properties/
│   │   ├── handler.go       # Property HTTP handlers
│   │   └── service.go       # Property and invite code management
│   ├── users/
│   │   ├── model.go         # User data structures
│   │   └── service.go       # User CRUD operations
│   └── utils/
│       ├── hash.go          # Password hashing (bcrypt)
│       └── jwt.go           # JWT utilities
├── template.yaml            # AWS SAM template
├── go.mod                   # Go module definition
├── Makefile                 # Build and deploy commands
└── README.md
```

## Quick Start

### Prerequisites

- Go 1.21+
- AWS CLI configured
- AWS SAM CLI
- Docker (for local testing)

### Installation

```bash
# Clone the repository
cd booking-villa-backend

# Download dependencies
make deps
make tidy

# Build
make build
```

### Local Development

```bash
# Start local API Gateway
make local

# The API will be available at http://localhost:3000
```

### Deploy to AWS

```bash
# First-time deployment (interactive)
make deploy

# Subsequent deployments
sam deploy
```

## API Endpoints

### Authentication

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/auth/send-otp` | Send OTP to phone | Public |
| POST | `/auth/verify-otp` | Verify OTP, get JWT | Public |
| POST | `/auth/login` | Password login | Public |
| POST | `/auth/refresh` | Refresh JWT token | JWT |

### Users

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/users` | List users (pending by default) | Admin |
| GET | `/users/{phone}` | Get user by phone | JWT |
| PATCH | `/users/{phone}/status` | Approve/reject user | Admin |
| POST | `/users/password` | Set/update password | JWT |

### Properties

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/properties` | Create property | Owner/Admin |
| GET | `/properties` | List user's properties | JWT |
| GET | `/properties/{id}` | Get property details | Public |
| POST | `/properties/{id}/invite-codes` | Generate invite code | Owner/Admin |
| POST | `/invite-codes/validate` | Validate invite code | Public |

### Bookings

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/bookings` | Create booking | Any Role |
| GET | `/bookings` | List bookings | JWT |
| GET | `/bookings/{id}` | Get booking | JWT |
| PATCH | `/bookings/{id}/status` | Update status | Owner/Admin |

### Payments

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/bookings/{id}/payments` | Log payment | Any Role |
| GET | `/bookings/{id}/payments` | List payments | JWT |
| GET | `/bookings/{id}/payment-status` | Get payment status | JWT |

## Business Rules

1. **Phone as Primary Identifier**: Users are identified by phone number
2. **OTP Expiry**: OTPs expire after 5 minutes
3. **User Approval**: Non-admin users require admin approval
4. **Invite Codes**: Property-specific codes for agent onboarding
5. **Offline Payments Only**: No online payment processing
6. **Payment Status Logic**:
   - `pending`: No payments recorded
   - `due`: Partial payment received
   - `completed`: Full amount paid

## DynamoDB Schema (Single-Table Design)

| Entity | PK | SK | GSI1PK | GSI1SK |
|--------|----|----|--------|--------|
| User | `USER#<phone>` | `PROFILE` | `ROLE#<role>` | `USER#<phone>` |
| OTP | `OTP#<phone>` | `CODE#<otp>` | - | - |
| Property | `PROPERTY#<id>` | `METADATA` | `OWNER#<userId>` | `PROPERTY#<id>` |
| InviteCode | `INVITE#<code>` | `PROPERTY#<propId>` | `PROPERTY#<propId>` | `INVITE#<code>` |
| Booking | `BOOKING#<id>` | `METADATA` | `PROPERTY#<propId>` | `DATE#<checkIn>` |
| Payment | `PAYMENT#<bookingId>` | `DATE#<date>#<id>` | `BOOKING#<bookingId>` | `PAYMENT#<id>` |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TABLE_NAME` | DynamoDB table name | `BookingPlatformTable` |
| `JWT_SECRET` | JWT signing secret | - |
| `OTP_EXPIRY_MINUTES` | OTP validity duration | `5` |

## License

MIT
