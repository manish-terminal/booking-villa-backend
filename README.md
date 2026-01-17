# Booking Villa Backend

A production-ready Go backend for a Hotel/Villa Booking Platform running on AWS Lambda with SAM.

## Features

- **Phone-based OTP Authentication** with 5-minute expiry
- **JWT Token Authentication** (24-hour validity)
- **Role-Based Access Control**: Admin, Owner, Agent
- **Property Management** with invite codes
- **Booking System** with availability checking
- **Offline Payment Tracking** (pending → due → completed)

## Quick Start

```bash
# Build
make build

# Deploy
sam deploy --guided
```

---

# API Reference

**Base URL**: `https://vwn08g3i79.execute-api.ap-south-1.amazonaws.com/prod`

---

## Authentication

### POST /auth/send-otp
Send OTP to phone number.

**Request:**
```json
{
  "phone": "9876543210"
}
```

**Response (200):**
```json
{
  "message": "OTP sent successfully",
  "phone": "9876543210",
  "code": "123456"
}
```

---

### POST /auth/verify-otp
Verify OTP and get JWT token. Auto-creates user if new.

**Request:**
```json
{
  "phone": "9876543210",
  "code": "123456",
  "name": "John Doe",
  "role": "agent"
}
```
*Valid roles: `admin`, `owner`, `agent`*

**Response (200):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "phone": "9876543210",
    "name": "John Doe",
    "role": "agent",
    "status": "approved",
    "createdAt": "2026-01-18T00:00:00Z",
    "updatedAt": "2026-01-18T00:00:00Z"
  },
  "isNew": true,
  "message": "Authentication successful"
}

---

### POST /auth/login
Login with phone and password.

**Request:**
```json
{
  "phone": "9876543210",
  "password": "yourpassword"
}
```

**Response (200):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {...},
  "message": "Login successful"
}
```

---

### POST /auth/refresh
Refresh JWT token.

**Headers:** `Authorization: Bearer <token>`

**Response (200):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {...},
  "message": "Token refreshed"
}
```

---

## Users

### POST /users/password
Set or update password.

**Headers:** `Authorization: Bearer <token>`

**Request:**
```json
{
  "password": "newpassword123",
  "oldPassword": "oldpassword"
}
```

**Response (200):**
```json
{
  "message": "Password set successfully"
}
```

---

### GET /users
List users (Admin only).

**Headers:** `Authorization: Bearer <token>`

**Query Params:** `?role=agent` (optional)

**Response (200):**
```json
{
  "users": [
    {
      "phone": "9876543210",
      "name": "John Doe",
      "role": "agent",
      "status": "pending",
      "createdAt": "2026-01-18T00:00:00Z",
      "updatedAt": "2026-01-18T00:00:00Z"
    }
  ],
  "count": 1
}
```

---

### GET /users/{phone}
Get user by phone.

**Headers:** `Authorization: Bearer <token>`

**Response (200):**
```json
{
  "phone": "9876543210",
  "name": "John Doe",
  "role": "admin",
  "status": "approved",
  "createdAt": "2026-01-18T00:00:00Z",
  "updatedAt": "2026-01-18T00:00:00Z"
}
```

---

### PATCH /users/{phone}/status
Approve or reject user (Admin only).

**Headers:** `Authorization: Bearer <token>`

**Request:**
```json
{
  "status": "approved"
}
```
*Valid statuses: `pending`, `approved`, `rejected`*

**Response (200):**
```json
{
  "message": "User status updated",
  "phone": "9876543210",
  "status": "approved"
}
```

---

## Properties

### POST /properties
Create a property (Owner/Admin).

**Headers:** `Authorization: Bearer <token>`

**Request:**
```json
{
  "name": "Sunset Beach Villa",
  "description": "Beautiful beachfront villa with pool",
  "address": "123 Beach Road",
  "city": "Goa",
  "state": "Goa",
  "country": "India",
  "pricePerNight": 5000,
  "currency": "INR",
  "maxGuests": 6,
  "bedrooms": 3,
  "bathrooms": 2,
  "amenities": ["wifi", "pool", "ac", "parking"],
  "images": ["https://example.com/img1.jpg"]
}
```

**Response (201):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Sunset Beach Villa",
  "description": "Beautiful beachfront villa with pool",
  "address": "123 Beach Road",
  "city": "Goa",
  "state": "Goa",
  "country": "India",
  "ownerId": "9876543210",
  "pricePerNight": 5000,
  "currency": "INR",
  "maxGuests": 6,
  "bedrooms": 3,
  "bathrooms": 2,
  "amenities": ["wifi", "pool", "ac", "parking"],
  "isActive": true,
  "createdAt": "2026-01-18T00:00:00Z",
  "updatedAt": "2026-01-18T00:00:00Z"
}
```

---

### GET /properties
List my properties.

**Headers:** `Authorization: Bearer <token>`

**Response (200):**
```json
{
  "properties": [...],
  "count": 2
}
```

---

### GET /properties/{id}
Get property by ID (Public).

**Response (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Sunset Beach Villa",
  ...
}
```

---

### POST /properties/{id}/invite-codes
Generate invite code for agents (Owner/Admin).

**Headers:** `Authorization: Bearer <token>`

**Request:**
```json
{
  "expiresInDays": 30,
  "maxUses": 10
}
```

**Response (201):**
```json
{
  "code": "a1b2c3d4",
  "propertyId": "550e8400-e29b-41d4-a716-446655440000",
  "propertyName": "Sunset Beach Villa",
  "createdBy": "9876543210",
  "createdAt": "2026-01-18T00:00:00Z",
  "expiresAt": "2026-02-17T00:00:00Z",
  "maxUses": 10,
  "usedCount": 0,
  "isActive": true
}
```

---

### POST /invite-codes/validate
Validate an invite code (Public).

**Request:**
```json
{
  "code": "a1b2c3d4"
}
```

**Response (200):**
```json
{
  "valid": true,
  "inviteCode": {...},
  "message": "Invite code is valid"
}
```

**Response (400):**
```json
{
  "error": "invite code has expired"
}
```

---

## Bookings

### POST /bookings
Create a booking.

**Headers:** `Authorization: Bearer <token>`

**Request:**
```json
{
  "propertyId": "550e8400-e29b-41d4-a716-446655440000",
  "guestName": "Jane Smith",
  "guestPhone": "9998887776",
  "guestEmail": "jane@example.com",
  "numGuests": 4,
  "checkIn": "2026-02-01",
  "checkOut": "2026-02-05",
  "notes": "Early check-in requested",
  "specialRequests": "Vegetarian meals",
  "inviteCode": "a1b2c3d4"
}
```

**Response (201):**
```json
{
  "id": "660e8400-e29b-41d4-a716-446655440001",
  "propertyId": "550e8400-e29b-41d4-a716-446655440000",
  "propertyName": "Sunset Beach Villa",
  "guestName": "Jane Smith",
  "guestPhone": "9998887776",
  "guestEmail": "jane@example.com",
  "numGuests": 4,
  "checkIn": "2026-02-01T00:00:00Z",
  "checkOut": "2026-02-05T00:00:00Z",
  "numNights": 4,
  "pricePerNight": 5000,
  "totalAmount": 20000,
  "currency": "INR",
  "status": "pending_confirmation",
  "bookedBy": "9876543210",
  "notes": "Early check-in requested",
  "createdAt": "2026-01-18T00:00:00Z",
  "updatedAt": "2026-01-18T00:00:00Z"
}
```

**Response (409 - Not Available):**
```json
{
  "error": "Property is not available for the selected dates"
}
```

---

### GET /bookings
List bookings.

**Headers:** `Authorization: Bearer <token>`

**Query Params:**
- `propertyId` (required)
- `startDate` (optional, format: YYYY-MM-DD)
- `endDate` (optional, format: YYYY-MM-DD)

**Response (200):**
```json
{
  "bookings": [...],
  "count": 5
}
```

---

### GET /bookings/{id}
Get booking by ID.

**Headers:** `Authorization: Bearer <token>`

**Response (200):**
```json
{
  "id": "660e8400-e29b-41d4-a716-446655440001",
  ...
}
```

---

### PATCH /bookings/{id}/status
Update booking status (Owner/Admin).

**Headers:** `Authorization: Bearer <token>`

**Request:**
```json
{
  "status": "confirmed"
}
```
*Valid statuses: `pending_confirmation`, `confirmed`, `checked_in`, `checked_out`, `cancelled`, `no_show`*

**Response (200):**
```json
{
  "message": "Booking status updated",
  "bookingId": "660e8400-e29b-41d4-a716-446655440001",
  "status": "confirmed"
}
```

---

## Payments

### POST /bookings/{id}/payments
Log an offline payment.

**Headers:** `Authorization: Bearer <token>`

**Request:**
```json
{
  "amount": 10000,
  "method": "upi",
  "reference": "UPI-TXN-12345",
  "notes": "Advance payment",
  "paymentDate": "2026-01-18"
}
```
*Valid methods: `cash`, `upi`, `bank_transfer`, `cheque`, `other`*

**Response (201):**
```json
{
  "payment": {
    "id": "770e8400-e29b-41d4-a716-446655440002",
    "bookingId": "660e8400-e29b-41d4-a716-446655440001",
    "amount": 10000,
    "currency": "INR",
    "method": "upi",
    "reference": "UPI-TXN-12345",
    "recordedBy": "9876543210",
    "paymentDate": "2026-01-18T00:00:00Z",
    "createdAt": "2026-01-18T00:00:00Z"
  },
  "summary": {
    "bookingId": "660e8400-e29b-41d4-a716-446655440001",
    "totalAmount": 20000,
    "totalPaid": 10000,
    "totalDue": 10000,
    "status": "due",
    "paymentCount": 1,
    "currency": "INR",
    "lastPaymentDate": "2026-01-18T00:00:00Z"
  },
  "message": "Payment logged successfully"
}
```

---

### GET /bookings/{id}/payments
Get all payments for a booking.

**Headers:** `Authorization: Bearer <token>`

**Response (200):**
```json
{
  "payments": [...],
  "count": 2
}
```

---

### GET /bookings/{id}/payment-status
Get payment status summary.

**Headers:** `Authorization: Bearer <token>`

**Response (200):**
```json
{
  "bookingId": "660e8400-e29b-41d4-a716-446655440001",
  "totalAmount": 20000,
  "totalPaid": 20000,
  "totalDue": 0,
  "status": "completed",
  "paymentCount": 2,
  "currency": "INR",
  "lastPaymentDate": "2026-01-20T00:00:00Z"
}
```

---

## Health Check

### GET /health

**Response (200):**
```json
{
  "status": "healthy",
  "service": "booking-villa-backend"
}
```

---

## Analytics

### GET /analytics/owner
Get owner analytics (Owner/Admin only).

**Headers:** `Authorization: Bearer <token>`

**Query Params:**
- `startDate` (optional, format: YYYY-MM-DD, default: start of month)
- `endDate` (optional, format: YYYY-MM-DD, default: end of month)

**Response (200):**
```json
{
  "totalProperties": 3,
  "totalBookings": 25,
  "totalRevenue": 125000,
  "totalCollected": 100000,
  "totalPending": 25000,
  "currency": "INR",
  "bookingsByStatus": {
    "confirmed": 15,
    "pending_confirmation": 5,
    "checked_out": 5
  },
  "paymentsByStatus": {
    "completed": 18,
    "due": 5,
    "pending": 2
  },
  "propertyStats": [
    {
      "propertyId": "550e8400-e29b-41d4-a716-446655440000",
      "propertyName": "Beach Villa",
      "totalBookings": 10,
      "totalRevenue": 50000,
      "totalCollected": 40000,
      "occupancyDays": 35
    }
  ],
  "periodStart": "2026-01-01T00:00:00Z",
  "periodEnd": "2026-01-31T23:59:59Z"
}
```

---

### GET /analytics/agent
Get agent analytics (any authenticated user).

**Headers:** `Authorization: Bearer <token>`

**Query Params:**
- `startDate` (optional)
- `endDate` (optional)

**Response (200):**
```json
{
  "totalBookings": 12,
  "totalBookingValue": 60000,
  "totalCollected": 45000,
  "currency": "INR",
  "bookingsByStatus": {
    "confirmed": 8,
    "pending_confirmation": 4
  },
  "recentBookings": [
    {
      "bookingId": "660e8400-e29b-41d4-a716-446655440001",
      "propertyName": "Beach Villa",
      "guestName": "John Doe",
      "checkIn": "2026-02-01T00:00:00Z",
      "checkOut": "2026-02-05T00:00:00Z",
      "totalAmount": 20000,
      "status": "confirmed",
      "paymentStatus": "due"
    }
  ],
  "periodStart": "2026-01-01T00:00:00Z",
  "periodEnd": "2026-01-31T23:59:59Z"
}
```

---

### GET /analytics/dashboard
Get quick dashboard stats.

**Headers:** `Authorization: Bearer <token>`

**Response (200):**
```json
{
  "todayCheckIns": 2,
  "todayCheckOuts": 1,
  "pendingApprovals": 3,
  "pendingPayments": 5,
  "totalDueAmount": 25000,
  "currency": "INR"
}
```

---

## Error Responses

All errors follow this format:
```json
{
  "error": "Error message here"
}
```

| Status | Meaning |
|--------|---------|
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Missing/invalid token |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found - Resource doesn't exist |
| 409 | Conflict - Resource conflict (e.g., dates unavailable) |
| 500 | Server Error |

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TABLE_NAME` | DynamoDB table name | `BookingPlatformTable` |
| `JWT_SECRET` | JWT signing secret | - |
| `OTP_EXPIRY_MINUTES` | OTP validity (minutes) | `5` |

---

## License

MIT License
