package commands

import "testing"

func Test_extractVIN(t *testing.T) {
	type args struct {
		hexValue string
	}
	tests := []struct {
		name    string
		args    args
		wantVin string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVin, err := extractVIN(tt.args.hexValue)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractVIN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotVin != tt.wantVin {
				t.Errorf("extractVIN() gotVin = %v, want %v", gotVin, tt.wantVin)
			}
		})
	}
}
