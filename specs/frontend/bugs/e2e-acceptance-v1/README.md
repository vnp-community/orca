# Bug nghiệm thu e2e: phần frontend (v1)

Phần giao diện của đợt sửa lỗi nghiệm thu e2e. Backend: [`specs/backend-go/bugs/e2e-acceptance-v1`](../../../backend-go/bugs/e2e-acceptance-v1/README.md).

1. [IMPACT-ASSESSMENT-FRONTEND.md](./IMPACT-ASSESSMENT-FRONTEND.md)
2. [solutions/](./solutions/README.md): 3 solution
3. [tasks/](./tasks/README.md): 8 task có checklist

Frontend **không** có bug chặn luồng tự thân; nó (a) che bug backend bằng cách nuốt lỗi và (b) thiếu hàng rào hợp đồng. Mục tiêu: **người dùng luôn thấy lý do và hướng khắc phục khi luồng bị chặn**, và lỗi hợp đồng kế tiếp bị bắt ở CI.
