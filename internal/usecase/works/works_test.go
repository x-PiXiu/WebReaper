package works

import (
	"testing"

	"webreaper/internal/domain/entity"
)

func TestIsDeliverableTask(t *testing.T) {
	tests := []struct {
		name    string
		task    entity.GenerationTask
		want    bool
	}{
		{
			name: "text2image is material",
			task: entity.GenerationTask{SubType: "text2image", ParamsJSON: `{}`},
			want: false,
		},
		{
			name: "tts is material",
			task: entity.GenerationTask{SubType: "tts", ParamsJSON: `{}`},
			want: false,
		},
		{
			name: "text2video is deliverable",
			task: entity.GenerationTask{SubType: "text2video", ParamsJSON: `{}`},
			want: true,
		},
		{
			name: "lip_sync is deliverable",
			task: entity.GenerationTask{SubType: "lip_sync", ParamsJSON: `{}`},
			want: true,
		},
		{
			name: "reference2video is deliverable",
			task: entity.GenerationTask{SubType: "reference2video", ParamsJSON: `{}`},
			want: true,
		},
		{
			name: "digital_human is deliverable",
			task: entity.GenerationTask{SubType: "digital_human", ParamsJSON: `{}`},
			want: true,
		},
		{
			name: "deliverable flag overrides unknown sub_type",
			task: entity.GenerationTask{SubType: "custom", ParamsJSON: `{"deliverable":true}`},
			want: true,
		},
		{
			name: "sub_type from params __sub_type",
			task: entity.GenerationTask{ParamsJSON: `{"__sub_type":"text2image"}`},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDeliverableTask(tt.task); got != tt.want {
				t.Fatalf("isDeliverableTask() = %v, want %v", got, tt.want)
			}
		})
	}
}
