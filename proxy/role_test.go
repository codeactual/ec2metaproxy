package proxy

import (
	"testing"
)

func TestNewRoleArn(t *testing.T) {
	arn, err := newRoleArn("arn:aws:iam::123456789012:role/test-role-name")
	if err != nil {
		t.Fatalf("unexpected err: %+v", err)
	}
	stringsEqual(t, [][2]string{
		[2]string{"test-role-name", arn.RoleName()},
		[2]string{"/", arn.Path()},
		[2]string{"123456789012", arn.AccountID()},
		[2]string{"arn:aws:iam::123456789012:role/test-role-name", arn.String()},
	})
}

func TestNewRoleArnWithPath(t *testing.T) {
	arn, err := newRoleArn("arn:aws:iam::123456789012:role/this/is/the/path/test-role-name")
	if err != nil {
		t.Fatalf("unexpected err: %+v", err)
	}
	stringsEqual(t, [][2]string{
		[2]string{"test-role-name", arn.RoleName()},
		[2]string{"/this/is/the/path/", arn.Path()},
		[2]string{"123456789012", arn.AccountID()},
		[2]string{"arn:aws:iam::123456789012:role/this/is/the/path/test-role-name", arn.String()},
	})
}
