package controller

import (
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/bacchus-snu/sgs/model"
	"github.com/bacchus-snu/sgs/pkg/auth"
)

func TestCheckNodegroups(t *testing.T) {
	tests := map[string]struct {
		groups    []string
		nodegroup string
		wantErr   error
	}{
		"undergraduate user can request undergraduate": {
			groups:    []string{string(model.NodegroupUndergraduate)},
			nodegroup: string(model.NodegroupUndergraduate),
		},
		"graduate user can request graduate": {
			groups:    []string{string(model.NodegroupGraduate)},
			nodegroup: string(model.NodegroupGraduate),
		},
		"professor user can request undergraduate": {
			groups:    []string{model.UserGroupProfessor},
			nodegroup: string(model.NodegroupUndergraduate),
		},
		"professor user can request graduate": {
			groups:    []string{model.UserGroupProfessor},
			nodegroup: string(model.NodegroupGraduate),
		},
		"undergraduate user cannot request graduate": {
			groups:    []string{string(model.NodegroupUndergraduate)},
			nodegroup: string(model.NodegroupGraduate),
			wantErr:   echo.ErrForbidden,
		},
		"invalid nodegroup is rejected": {
			groups:    []string{model.UserGroupProfessor},
			nodegroup: "invalid",
			wantErr:   echo.ErrBadRequest,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			user := &auth.User{Groups: tt.groups}
			if gotErr := checkNodegroups(user, tt.nodegroup); gotErr != tt.wantErr {
				t.Fatalf("checkNodegroups() = %v; want %v", gotErr, tt.wantErr)
			}
		})
	}
}
