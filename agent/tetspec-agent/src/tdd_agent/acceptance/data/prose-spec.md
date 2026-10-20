# Notification service - software requirements

## 1. Purpose

The notification service tells asset owners that a license entitlement is about to expire. This document is written as prose; it has no requirement tables.

## 2. Behaviour

The service shall send a reminder to the asset owner 60 days, 30 days and 7 days before an entitlement expires.
A reminder must be sent exactly once per milestone, even when the daily job runs twice.
When an owner renews an entitlement more than 60 days ahead, the service must clear the milestones that were already reminded.
The notification API shall answer within 2 seconds for at least 95 percent of requests.
Delivery of an email shall be retried 3 times with a delay of 5 minutes between attempts if the mail gateway returns an error.
The service must never store the content of the license document itself.
If the owner has no email address, the reminder is posted to the project channel instead.

## 3. Operations

The service exposes GET /api/v1/reminders to list pending reminders for the calling owner.
The request is rejected with HTTP 401 when the caller is not authenticated.

## 4. Notes

This section only contains background text without any obligation.
