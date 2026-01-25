# Frontend Development Prompt

## Context
We have updated the backend with two new capabilities for the Agent experience:
1.  **Property Performance Analytics**: An endpoint to get revenue and commission breakdown by property.
2.  **Availability Search**: An endpoint to find available properties for specific dates.

## Task 1: Agent Dashboard - Performance Chart
**Objective**: Add a "Property Performance" section to the Agent Dashboard.
- **Endpoint**: `GET /analytics/agent/property-performance?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD`
- **Response**: `{ "data": [{ "propertyId": "...", "propertyName": "...", "totalRevenue": 150000, "totalCommission": 15000, "bookingCount": 3 }] }`
- **UI Requirements**:
  - Implement a Bar Chart (using Recharts or similar).
  - X-Axis: Property Names.
  - Y-Axis: Currency Amount.
  - Bars: Show "Total Revenue" and "Commission" side-by-side for each property.
  - Tooltip: Show Revenue, Commission, and Booking Count on hover.
  - Include a date range picker to filter the data.

## Task 2: New Booking Flow - Availability Search
**Objective**: Create a "Find Available Properties" feature for agents to start a booking.
- **Endpoint**: `GET /properties/available?checkIn=YYYY-MM-DD&checkOut=YYYY-MM-DD`
- **Response**: `{ "properties": [{ "id": "...", "name": "...", "images": [...], "pricePerNight": ... }] }`
- **UI Requirements**:
  - **Search Form**: Inputs for Check-in Date and Check-out Date.
  - **Results List**: Display available properties as cards (Image, Name, Price, Location).
  - **Action**: "Book Now" button on each card that navigates to the booking form with pre-filled Property ID and Dates.
  - **Empty State**: "No properties available for these dates."

## Implementation Notes
- Ensure all API calls use the authenticated `api` client (Authorized header).
- Handle loading states (skeletons/spinners) while fetching data.
- Handle error states (e.g., "Failed to load properties").
