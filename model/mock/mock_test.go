package mock

import (
	"testing"

	"github.com/bacchus-snu/sgs/model"
	"github.com/bacchus-snu/sgs/model/test"
)

func TestMock(t *testing.T) {
	test.TestWorkspace(t, func() model.WorkspaceService {
		return New().Workspaces
	})
}
