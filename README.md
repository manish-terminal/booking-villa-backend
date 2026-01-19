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

## Endpoint Summary by Role

### 1. Shared Endpoints (Owner & Agent)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/auth/send-otp` | POST | Initial login/signup trigger |
| `/auth/check-user` | GET | Check if user exists before OTP |
| `/auth/verify-otp` | POST | Authentication & Role creation |
| `/auth/login` | POST | Password-based authentication |
| `/auth/refresh` | POST | Refresh JWT token |
| `/users/password` | POST | Account security management |
| `/properties` | GET | Owners see all; Agents see linked villas |
| `/properties/{id}` | GET | Get property details |
| `/properties/{id}/calendar` | GET | Checking room availability |
| `/properties/{id}/availability` | GET | Check specific date availability |
| `/bookings` | POST | Finalizing a reservation |
| `/bookings` | GET | Viewing lists of current stays |
| `/bookings/{id}` | GET | Get booking details |
| `/bookings/{id}/status` | PATCH | Marking Check-in / Check-out |
| `/analytics/dashboard` | GET | Snapshot of today's arrivals/departures |

### 2. Owner-Only Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/properties` | POST | Adding a new villa to the portfolio |
| `/properties/{id}` | PATCH | Updating pricing or description |
| `/properties/{id}/invite-codes` | POST | Creating keys to onboard new Agents |
| `/properties/{id}/invite-codes` | GET | Managing existing agent access codes |
| `/bookings/{id}/payments` | POST | Financial Settlement: Logging guest payments |
| `/bookings/{id}/payments` | GET | Transaction auditing |
| `/bookings/{id}/payment-status` | GET | Payment status summary |
| `/analytics/owner` | GET | Full revenue & performance reporting |

### 3. Agent-Only Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/invite-codes/validate` | POST | Checking if an Owner's code is valid to join |
| `/analytics/agent` | GET | Tracking personal commission & collections |

### 4. Admin-Only Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/users` | GET | List all users |
| `/users/{phone}` | GET | Get user by phone |
| `/users/{phone}/status` | PATCH | Approve or reject user |

---

# 1. Shared Endpoints (Owner & Agent)

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

### GET /auth/check-user
Check if a user exists before initiating OTP flow.

**Query Params:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `phone` | string | Yes | 10-digit phone number |

**Response (200 - User Exists):**
```json
{
  "exists": true,
  "hasPassword": true,
  "role": "owner",
  "status": "approved"
}
```

**Response (200 - User Does Not Exist):**
```json
{
  "exists": false
}
```

> [!TIP]
> Use this endpoint to determine the login flow:
> - If `exists: false` → Show registration form after OTP
> - If `exists: true` and `hasPassword: true` → Offer password login option
> - If `exists: true` and `hasPassword: false` → Proceed with OTP only

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

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `phone` | string | Yes | 10-digit phone number |
| `code` | string | Yes | 6-digit OTP |
| `name` | string | No | User's display name (required if new user) |
| `role` | string | No | `admin`, `owner`, or `agent` (required if new user) |

**Response (200):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "phone": "9876543210",
    "name": "John Doe",
    "role": "agent",
    "status": "approved",
    "managedProperties": [],
    "createdAt": "2026-01-18T00:00:00Z",
    "updatedAt": "2026-01-18T00:00:00Z"
  },
  "isNew": true,
  "message": "Authentication successful"
}
```

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
  "user": {
    "phone": "9876543210",
    "name": "John Doe",
    "role": "owner",
    "status": "approved"
  },
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

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `password` | string | Yes | New password (min 6 characters) |
| `oldPassword` | string | No | Required only when updating existing password |

**Response (200):**
```json
{
  "message": "Password set successfully"
}
```

---

## Properties

### GET /properties
List properties accessible to the current user.

**Headers:** `Authorization: Bearer <token>`

**Behavior:**
- **Owner**: Returns all properties owned by the user
- **Agent**: Returns properties linked via invite codes
- **Admin**: Returns all properties

**Response (200):**
```json
{
  "properties": [
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
  ],
  "count": 1
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

### GET /properties/{id}/availability
Check if a property is available for specific dates.

**Query Params:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `checkIn` | string | Yes | Format: YYYY-MM-DD |
| `checkOut` | string | Yes | Format: YYYY-MM-DD |

**Response (200):**
```json
{
  "propertyId": "550e8400-e29b-41d4-a716-446655440000",
  "checkIn": "2026-02-01",
  "checkOut": "2026-02-05",
  "available": true
}
```

---

### GET /properties/{id}/calendar
Get simplified list of occupied date ranges for a calendar view.

**Query Params:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `startDate` | string | No | Format: YYYY-MM-DD (default: start of current month) |
| `endDate` | string | No | Format: YYYY-MM-DD (default: end of current month) |

**Response (200):**
```json
{
  "propertyId": "550e8400-e29b-41d4-a716-446655440000",
  "startDate": "2026-02-01",
  "endDate": "2026-02-28",
  "occupied": [
    {
      "checkIn": "2026-02-10T00:00:00Z",
      "checkOut": "2026-02-15T00:00:00Z",
      "status": "confirmed"
    }
  ]
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
  "inviteCode": "a1b2c3d4",
  "pricePerNight": 5000,
  "totalAmount": 20000,
  "agentCommission": 1000
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `propertyId` | string | Yes | UUID of the property |
| `guestName` | string | Yes | Guest's full name |
| `guestPhone` | string | Yes | Guest's phone number |
| `guestEmail` | string | No | Guest's email |
| `numGuests` | int | Yes | Number of guests |
| `checkIn` | string | Yes | Format: YYYY-MM-DD |
| `checkOut` | string | Yes | Format: YYYY-MM-DD |
| `notes` | string | No | Internal notes |
| `specialRequests` | string | No | Guest requests |
| `inviteCode` | string | No | Agent's invite code |
| `pricePerNight` | int | No | Override property price |
| `totalAmount` | int | No | Override calculated total |
| `agentCommission` | int | No | Agent commission amount |

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
  "agentCommission": 1000,
  "currency": "INR",
  "status": "pending_confirmation",
  "bookedBy": "9876543210",
  "notes": "Early check-in requested",
  "createdAt": "2026-01-18T00:00:00Z",
  "updatedAt": "2026-01-18T00:00:00Z"
}
```

**Response (409 - Conflict):**
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

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `propertyId` | string | Yes | Filter by property ID |
| `startDate` | string | No | Format: YYYY-MM-DD |
| `endDate` | string | No | Format: YYYY-MM-DD |

**Response (200):**
```json
{
  "bookings": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "propertyId": "550e8400-e29b-41d4-a716-446655440000",
      "propertyName": "Sunset Beach Villa",
      "guestName": "Jane Smith",
      "guestPhone": "9998887776",
      "checkIn": "2026-02-01T00:00:00Z",
      "checkOut": "2026-02-05T00:00:00Z",
      "numNights": 4,
      "totalAmount": 20000,
      "status": "confirmed",
      "bookedBy": "9876543210",
      "createdAt": "2026-01-18T00:00:00Z"
    }
  ],
  "count": 1
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
  "agentCommission": 1000,
  "currency": "INR",
  "status": "confirmed",
  "bookedBy": "9876543210",
  "notes": "Early check-in requested",
  "specialRequests": "Vegetarian meals",
  "createdAt": "2026-01-18T00:00:00Z",
  "updatedAt": "2026-01-18T00:00:00Z"
}
```

---

### PATCH /bookings/{id}/status
Update booking status.

**Headers:** `Authorization: Bearer <token>`

**Request:**
```json
{
  "status": "confirmed"
}
```

| Status | Description |
|--------|-------------|
| `pending_confirmation` | Awaiting owner approval |
| `confirmed` | Booking confirmed |
| `checked_in` | Guest has arrived |
| `checked_out` | Guest has departed |
| `cancelled` | Booking cancelled |
| `no_show` | Guest did not arrive |

**Response (200):**
```json
{
  "message": "Booking status updated",
  "bookingId": "660e8400-e29b-41d4-a716-446655440001",
  "status": "confirmed"
}
```

---

### GET /analytics/dashboard
Get quick dashboard stats for today.

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

# 2. Owner-Only Endpoints

## Properties

### POST /properties
Create a new property.

**Headers:** `Authorization: Bearer <token>`  
**Required Role:** Owner or Admin

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

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Property name |
| `description` | string | No | Detailed description |
| `address` | string | Yes | Street address |
| `city` | string | Yes | City name |
| `state` | string | Yes | State/Province |
| `country` | string | No | Country (default: India) |
| `pricePerNight` | int | Yes | Price per night |
| `currency` | string | No | Currency code (default: INR) |
| `maxGuests` | int | Yes | Maximum guest capacity |
| `bedrooms` | int | No | Number of bedrooms |
| `bathrooms` | int | No | Number of bathrooms |
| `amenities` | array | No | List of amenities |
| `images` | array | No | List of image URLs |

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

### PATCH /properties/{id}
Update property details.

**Headers:** `Authorization: Bearer <token>`  
**Required Role:** Owner (of property) or Admin

**Request:**
```json
{
  "name": "Updated Villa Name",
  "pricePerNight": 6000,
  "isActive": false
}
```

*All fields are optional. Only include fields you want to update.*

**Response (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Updated Villa Name",
  "pricePerNight": 6000,
  "isActive": false,
  "updatedAt": "2026-01-18T12:00:00Z"
}
```

---

## Invite Codes

### POST /properties/{id}/invite-codes
Generate an invite code for agents.

**Headers:** `Authorization: Bearer <token>`  
**Required Role:** Owner (of property) or Admin

**Request:**
```json
{
  "expiresInDays": 30,
  "maxUses": 10
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `expiresInDays` | int | No | 30 | Days until code expires |
| `maxUses` | int | No | 0 (unlimited) | Maximum number of uses |

**Response (201):**
```json
{
  "code": "A1B2C3D4",
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

### GET /properties/{id}/invite-codes
List all invite codes for a property.

**Headers:** `Authorization: Bearer <token>`  
**Required Role:** Owner (of property) or Admin

**Response (200):**
```json
{
  "inviteCodes": [
    {
      "code": "A1B2C3D4",
      "propertyId": "550e8400-e29b-41d4-a716-446655440000",
      "propertyName": "Sunset Beach Villa",
      "createdBy": "9876543210",
      "createdAt": "2026-01-18T00:00:00Z",
      "expiresAt": "2026-02-17T00:00:00Z",
      "maxUses": 10,
      "usedCount": 3,
      "isActive": true
    }
  ],
  "count": 1
}
```

---

## Payments

### POST /bookings/{id}/payments
Log an offline payment.

**Headers:** `Authorization: Bearer <token>`  
**Required Role:** Owner or Admin

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

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `amount` | int | Yes | Payment amount |
| `method` | string | Yes | `cash`, `upi`, `bank_transfer`, `cheque`, `other` |
| `reference` | string | No | Transaction reference ID |
| `notes` | string | No | Payment notes |
| `paymentDate` | string | No | Format: YYYY-MM-DD (default: today) |

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
  "payments": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "bookingId": "660e8400-e29b-41d4-a716-446655440001",
      "amount": 10000,
      "currency": "INR",
      "method": "upi",
      "reference": "UPI-TXN-12345",
      "recordedBy": "9876543210",
      "paymentDate": "2026-01-18T00:00:00Z",
      "createdAt": "2026-01-18T00:00:00Z"
    }
  ],
  "count": 1
}
```

---

### GET /bookings/{id}/payment-status
Get payment status summary for a booking.

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

| Payment Status | Description |
|----------------|-------------|
| `pending` | No payments recorded |
| `due` | Partial payment made |
| `completed` | Full amount paid |

---

## Owner Analytics

### GET /analytics/owner
Get comprehensive owner analytics.

**Headers:** `Authorization: Bearer <token>`  
**Required Role:** Owner or Admin

**Query Params:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `startDate` | string | No | Format: YYYY-MM-DD (default: start of month) |
| `endDate` | string | No | Format: YYYY-MM-DD (default: end of month) |

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

# 3. Agent-Only Endpoints

### POST /invite-codes/validate
Validate an invite code and link the property to the agent's account.

**Headers:** `Authorization: Bearer <token>`  
**Required Role:** Agent

**Request:**
```json
{
  "code": "A1B2C3D4"
}
```

*Note: Code is case-insensitive.*

**Response (200):**
```json
{
  "valid": true,
  "inviteCode": {
    "code": "a1b2c3d4",
    "propertyId": "550e8400-e29b-41d4-a716-446655440000",
    "propertyName": "Sunset Beach Villa",
    "expiresAt": "2026-02-17T00:00:00Z"
  },
  "message": "Invite code validated and property linked successfully"
}
```

**Error Responses:**

| Status | Error |
|--------|-------|
| 400 | `invite code has expired` |
| 400 | `invite code has reached maximum uses` |
| 404 | `invite code not found` |

---

### GET /analytics/agent
Get agent-specific analytics (commissions, bookings).

**Headers:** `Authorization: Bearer <token>`

**Query Params:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `startDate` | string | No | Format: YYYY-MM-DD (default: start of month) |
| `endDate` | string | No | Format: YYYY-MM-DD (default: end of month) |

**Response (200):**
```json
{
  "agentName": "John Agent",
  "agentPhone": "9876543210",
  "totalBookings": 12,
  "totalBookingValue": 60000,
  "totalCollected": 45000,
  "totalCommission": 3000,
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
      "agentCommission": 1000,
      "status": "confirmed",
      "paymentStatus": "due"
    }
  ],
  "periodStart": "2026-01-01T00:00:00Z",
  "periodEnd": "2026-01-31T23:59:59Z"
}
```

---

# 4. Admin-Only Endpoints

### GET /users
List all users.

**Headers:** `Authorization: Bearer <token>`  
**Required Role:** Admin

**Query Params:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `role` | string | No | Filter by role: `admin`, `owner`, `agent` |

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
**Required Role:** Admin or self

**Response (200):**
```json
{
  "phone": "9876543210",
  "name": "John Doe",
  "role": "admin",
  "status": "approved",
  "managedProperties": ["550e8400-e29b-41d4-a716-446655440000"],
  "createdAt": "2026-01-18T00:00:00Z",
  "updatedAt": "2026-01-18T00:00:00Z"
}
```

---

### PATCH /users/{phone}/status
Approve or reject a user.

**Headers:** `Authorization: Bearer <token>`  
**Required Role:** Admin

**Request:**
```json
{
  "status": "approved"
}
```

| Status | Description |
|--------|-------------|
| `pending` | Awaiting approval |
| `approved` | User can access the system |
| `rejected` | User access denied |

**Response (200):**
```json
{
  "message": "User status updated",
  "phone": "9876543210",
  "status": "approved"
}
```

---

# 5. Health Check

### GET /health

**Response (200):**
```json
{
  "status": "healthy",
  "service": "booking-villa-backend"
}
```

---

# Error Responses

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

# Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TABLE_NAME` | DynamoDB table name | `BookingPlatformTable` |
| `JWT_SECRET` | JWT signing secret | - |
| `OTP_EXPIRY_MINUTES` | OTP validity (minutes) | `5` |

---

# License

MIT License
