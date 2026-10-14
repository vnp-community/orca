<!-- orca:begin plan request=REQ-7 schema=1 -->
## TASKS
- [ ] T1.1 Sửa lỗi làm tròn [type=bug] [h=1] [ac=AC-1]
  Serves: BRL-03
  Code: modify `src/round.go::Round`
  Verify: AC-1 qua `go test`
  Done when: kết quả đúng.
- [ ] T1.2 Thêm test hồi quy [h=0.5] [labels=test:regression] [depends=T1.1]
  Serves: BRL-03
  Code: create `src/round_test.go::TestRound`
  Verify: AC-1 qua `go test`
  Done when: test xanh.
<!-- orca:end plan -->
