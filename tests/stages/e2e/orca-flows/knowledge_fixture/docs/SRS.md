# Notes service: Software Requirements Specification (SRS) v1.0

## 1. Introduction

### 1.1. Purpose

This SRS specifies the Notes service.

## 5. Functional requirements

### 5.1. Notes (FR-NOTE)

| Mã FR | Yêu cầu | Ưu tiên | UC | BRL | Thực thể (ENT) | API | Nghiệm thu |
| --- | --- | --- | --- | --- | --- | --- | --- |
| FR-NOTE-01 | The service shall store a note of at most 280 characters sent with POST /notes. | M | UC-01 | BRL-01 | ENT-01 | POST /notes | A note of 281 characters is refused with 422. |
| FR-NOTE-02 | The service shall return stored notes with GET /notes. | M | UC-02 | BRL-01 | ENT-01 | GET /notes | The notes stored earlier are returned in order. |
| FR-NOTE-03 | The service should answer GET /healthz with the status of the process. | S | UC-03 | BRL-02 | ENT-01 | GET /healthz | The answer has status ok while the process runs. |

## 6. Business rules

| Mã | Quy tắc | Nguồn |
| --- | --- | --- |
| BRL-01 | A note is text of 1 to 280 characters. | FR-NOTE-01 |
| BRL-02 | A health answer never reads the note store, so it cannot fail because of it. | FR-NOTE-03 |

## 7. Entities

| Mã | Thực thể | Thuộc tính |
| --- | --- | --- |
| ENT-01 | Note | id, text, created_at |
