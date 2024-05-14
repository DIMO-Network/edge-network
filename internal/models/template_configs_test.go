package models

import "testing"

func TestPIDRequest_FormulaType(t *testing.T) {
	type fields struct {
		Formula              string
		Header               uint32
		IntervalSeconds      int
		Mode                 uint32
		Name                 string
		Pid                  uint32
		Protocol             string
		CanflowControlClear  bool
		CanFlowControlIDPair string
	}
	tests := []struct {
		name   string
		fields fields
		want   FormulaType
	}{
		{
			name: "dbc formula type",
			fields: fields{
				Formula: `dbc:31|8@0+ (1,-40) [-40|215] "Celcius"`,
			},
			want: Dbc,
		},
		{
			name: "python formula type",
			fields: fields{
				Formula: `python:(bytes_to_int(messages[0].data[-1:]) - 50) * 1.8 + 32`,
			},
			want: Python,
		},
		{
			name: "unknown formula type",
			fields: fields{
				Formula: `(bytes_to_int(messages[0].data[-1:]) - 50) * 1.8 + 32`,
			},
			want: Unknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PIDRequest{
				Formula:              tt.fields.Formula,
				Header:               tt.fields.Header,
				IntervalSeconds:      tt.fields.IntervalSeconds,
				Mode:                 tt.fields.Mode,
				Name:                 tt.fields.Name,
				Pid:                  tt.fields.Pid,
				Protocol:             tt.fields.Protocol,
				CanflowControlClear:  tt.fields.CanflowControlClear,
				CanFlowControlIDPair: tt.fields.CanFlowControlIDPair,
			}
			if got := p.FormulaType(); got != tt.want {
				t.Errorf("FormulaType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPIDRequest_FormulaValue(t *testing.T) {
	type fields struct {
		Formula              string
		Header               uint32
		IntervalSeconds      int
		Mode                 uint32
		Name                 string
		Pid                  uint32
		Protocol             string
		CanflowControlClear  bool
		CanFlowControlIDPair string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "dbc formula type",
			fields: fields{
				Formula: `dbc:31|8@0+ (1,-40) [-40|215] "Celcius"`,
			},
			want: `31|8@0+ (1,-40) [-40|215] "Celcius"`,
		},
		{
			name: "python formula type",
			fields: fields{
				Formula: `python:(bytes_to_int(messages[0].data[-1:]) - 50) * 1.8 + 32`,
			},
			want: `(bytes_to_int(messages[0].data[-1:]) - 50) * 1.8 + 32`,
		},
		{
			name: "no formula type",
			fields: fields{
				Formula: `bytes_to_int(messages[0].data[-1:]) - 50`,
			},
			want: `bytes_to_int(messages[0].data[-1:]) - 50`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PIDRequest{
				Formula:              tt.fields.Formula,
				Header:               tt.fields.Header,
				IntervalSeconds:      tt.fields.IntervalSeconds,
				Mode:                 tt.fields.Mode,
				Name:                 tt.fields.Name,
				Pid:                  tt.fields.Pid,
				Protocol:             tt.fields.Protocol,
				CanflowControlClear:  tt.fields.CanflowControlClear,
				CanFlowControlIDPair: tt.fields.CanFlowControlIDPair,
			}
			if got := p.FormulaValue(); got != tt.want {
				t.Errorf("FormulaValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
