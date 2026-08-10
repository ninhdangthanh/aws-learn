package mq

import "testing"

func TestActionForTick(t *testing.T) {
	cases := []struct {
		name string
		tick int
		want TickAction
	}{
		{"message mới từ producer", 0, ActionProcess},
		{"mốc 30s", 1, ActionProcess},
		{"mốc 1m", 2, ActionProcess},
		{"mốc 2m", 4, ActionProcess},
		{"mốc 5m", 10, ActionProcess},
		{"mốc 10m", 20, ActionProcess},
		{"mốc 30m", 60, ActionProcess},
		{"giữa 1m và 2m", 3, ActionWait},
		{"giữa 2m và 5m", 7, ActionWait},
		{"giữa 10m và 30m", 45, ActionWait},
		{"ngay sau mốc cuối", 61, ActionDLQ},
		{"quá xa mốc cuối", 100, ActionDLQ},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ActionForTick(c.tick); got != c.want {
				t.Errorf("ActionForTick(%d) = %v, muốn %v", c.tick, got, c.want)
			}
		})
	}
}

// Message phải đi hết toàn bộ thang mà không kẹt: đúng 6 lần xử lý (chưa kể
// lần đầu tick=0) rồi vào DLQ, không có tick nào rơi ra ngoài 3 nhánh.
func TestThangRetryDiHetVaKetThucODLQ(t *testing.T) {
	processed := 0
	tick := 0

	for range 1000 {
		switch ActionForTick(tick) {
		case ActionProcess:
			processed++
			tick++
		case ActionWait:
			tick++
		case ActionDLQ:
			if processed != len(RetryTickMarks)+1 {
				t.Fatalf("xử lý %d lần trước khi vào DLQ, muốn %d", processed, len(RetryTickMarks)+1)
			}
			if want := RetryTickMarks[len(RetryTickMarks)-1] + 1; tick != want {
				t.Fatalf("vào DLQ ở tick=%d, muốn %d", tick, want)
			}
			return
		}
	}

	t.Fatalf("không bao giờ vào DLQ, kẹt ở tick=%d", tick)
}

// Header do producer đặt phải sống sót qua mỗi vòng retry.
func TestWithTickGiuNguyenHeaderKhac(t *testing.T) {
	orig := map[string]any{
		HeaderEventID: "evt_1",
		"trace_id":    "abc",
		HeaderTick:    int32(3),
	}

	next := WithTick(orig, 4)

	if got := TickOf(next); got != 4 {
		t.Errorf("tick = %d, muốn 4", got)
	}
	if next["trace_id"] != "abc" {
		t.Errorf("mất header trace_id: %+v", next)
	}
	if TickOf(orig) != 3 {
		t.Errorf("WithTick đã sửa vào table gốc: %+v", orig)
	}
}
