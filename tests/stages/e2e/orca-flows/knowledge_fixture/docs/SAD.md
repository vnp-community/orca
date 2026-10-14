# Notes service: Software Architecture Document (SAD) v1.0

## 1. Introduction

### 1.1. Purpose

This SAD describes the architecture of the Notes service.

## 4. Decisions

| Mã | Quyết định | Hệ quả |
| --- | --- | --- |
| ADR-01 | The service is one Go process with an in-memory store. | No external dependency; notes are lost on restart. |
| ADR-02 | Handlers live in main.go and are tested with net/http/httptest. | No framework. |
