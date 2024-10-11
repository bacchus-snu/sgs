package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bacchus-snu/sgs/model"
)

type (
	workspaceEvent string
	workspace      string
)

var setupLog = ctrl.Log.WithName("setup")

func main() {
	ctrl.SetLogger(zap.New())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		setupLog.Error(err, "failed creating manager")
		os.Exit(1)
	}

	if err := (&workspaceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "failed setting up controller")
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "failed starting manager")
		os.Exit(1)
	}
}

type workspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	WSSvc  model.WorkspaceService
}

func (r *workspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	setupLog.Info("setting up controller")

	events := make(chan event.TypedGenericEvent[workspaceEvent])

	go func() {
		// TODO: test
		time.Sleep(time.Second)
		events <- event.TypedGenericEvent[workspaceEvent]{
			Object: workspaceEvent("ws-gtzu3h6dr2ap3"),
		}
	}()

	wsHandler := handler.TypedEnqueueRequestsFromMapFunc(
		func(ctx context.Context, event workspaceEvent) []workspace {
			var ws workspace
			fmt.Sscanf(string(event), "ws-%s", &ws)
			setupLog.Info("enqueue1", "workspace", ws)
			return []workspace{ws}
		})

	wsResourceHandler := handler.TypedEnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []workspace {
			if wsName, ok := obj.GetLabels()["sgs.snucse.org/name"]; ok {
				var ws workspace
				fmt.Sscanf(string(wsName), "ws-%s", &ws)
				setupLog.Info("enqueue2", "workspace", ws)
				return []workspace{ws}
			}
			return nil
		})

	pred, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      "sgs.snucse.org/name",
			Operator: metav1.LabelSelectorOpExists,
		}},
	})
	if err != nil {
		return err
	}
	opts := builder.WithPredicates(pred)

	return builder.TypedControllerManagedBy[workspace](mgr).
		Named("workspace").
		WatchesRawSource(source.TypedChannel(events, wsHandler)).
		Watches(&corev1.Namespace{}, wsResourceHandler, opts).
		Watches(&corev1.ResourceQuota{}, wsResourceHandler, opts).
		Watches(&corev1.LimitRange{}, wsResourceHandler, opts).
		Watches(&rbacv1.RoleBinding{}, wsResourceHandler, opts).
		Complete(r)
}

func (r *workspaceReconciler) Reconcile(ctx context.Context, wsName workspace) (ctrl.Result, error) {
	ctxLog := log.FromContext(ctx)
	ctxLog.Info("reconcile triggered", "workspace", wsName)

	wsID, err := model.ParseID(string(wsName))
	if err != nil {
		// wsID is invalid, ignore
		return ctrl.Result{}, reconcile.TerminalError(err)
	}

	ws, err := r.WSSvc.GetWorkspace(ctx, wsID)
	if err != nil {
		// TODO: delete if not found
		return ctrl.Result{}, err
	}

	ctxLog.Info("workspace", "ws", ws)

	objs := constructResources(ws)

	for _, obj := range objs {
		ctxLog.Info("CREATING", "object", obj)
		err := r.Patch(ctx, obj, client.Apply, client.FieldOwner("sgs-controller"), client.ForceOwnership)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func constructResources(ws *model.Workspace) []client.Object {
	if !ws.Created {
		return nil
	}
	return []client.Object{
		constructNamespace(ws),
		constructResourceQuota(ws),
		constructLimitRange(ws),
		constructRoleBinding(ws),
	}
}

func constructObjectMeta(ws *model.Workspace) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: fmt.Sprintf("ws-%s", ws.Hash()),
		Name:      fmt.Sprintf("ws-%s", ws.Hash()),
		Labels: map[string]string{
			"sgs.snucse.org/id":   fmt.Sprintf("%d", ws.ID),
			"sgs.snucse.org/name": fmt.Sprintf("ws-%s", ws.Hash()),
		},
	}
}

func constructNamespace(ws *model.Workspace) *corev1.Namespace {
	meta := constructObjectMeta(ws)
	meta.Annotations = map[string]string{
		"scheduler.alpha.kubernetes.io/node-selector": fmt.Sprintf("node-restriction.kubernetes.io/nodegroup=%s", ws.Nodegroup),
	}
	return &corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: meta,
	}
}

// Names of resources managed by the controller
var managedResourceNames = []corev1.ResourceName{
	corev1.ResourceName(model.ResCPURequest),
	corev1.ResourceName(model.ResCPULimit),
	corev1.ResourceName(model.ResMemoryRequest),
	corev1.ResourceName(model.ResMemoryLimit),
	corev1.ResourceName(model.ResStorageRequest),
	corev1.ResourceName(model.ResGPURequest),
	// extra restricted resources, always 0
	"requests.ephemeral-storage",
	"services.loadbalancers",
	"services.nodeports",
}

func constructResourceQuota(ws *model.Workspace) *corev1.ResourceQuota {
	obj := &corev1.ResourceQuota{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ResourceQuota"},
		ObjectMeta: constructObjectMeta(ws),
	}

	obj.Spec.Hard = corev1.ResourceList{}
	for _, res := range managedResourceNames {
		obj.Spec.Hard[res] = *resource.NewQuantity(0, resource.DecimalSI)
	}
	if ws.Enabled {
		for res, qty := range ws.Quotas {
			switch res {
			case model.ResMemoryLimit, model.ResMemoryRequest, model.ResStorageRequest:
				obj.Spec.Hard[corev1.ResourceName(res)] = *resource.NewQuantity(int64(qty)<<30, resource.BinarySI)
			default:
				obj.Spec.Hard[corev1.ResourceName(res)] = *resource.NewQuantity(int64(qty), resource.DecimalSI)
			}
		}
	}

	return obj
}

func constructLimitRange(ws *model.Workspace) *corev1.LimitRange {
	obj := &corev1.LimitRange{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "LimitRange"},
		ObjectMeta: constructObjectMeta(ws),
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(10<<30, resource.BinarySI),
					},
					Max: corev1.ResourceList{
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(10<<30, resource.BinarySI),
					},
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU:              *resource.NewQuantity(0, resource.DecimalSI),
						corev1.ResourceMemory:           *resource.NewQuantity(0, resource.DecimalSI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(0, resource.DecimalSI),
					},
				},
			},
		},
	}

	if ws.Enabled {
		obj.Spec.Limits[0].Default[corev1.ResourceCPU] = *resource.NewQuantity(int64(ws.Quotas[model.ResCPULimit]), resource.DecimalSI)
		obj.Spec.Limits[0].Default[corev1.ResourceMemory] = *resource.NewQuantity(int64(ws.Quotas[model.ResMemoryLimit])<<30, resource.BinarySI)
	}

	return obj
}

func constructRoleBinding(ws *model.Workspace) *rbacv1.RoleBinding {
	obj := &rbacv1.RoleBinding{
		TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "RoleBinding"},
		ObjectMeta: constructObjectMeta(ws),
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "edit",
		},
		Subjects: []rbacv1.Subject{},
	}

	if ws.Enabled {
		for _, user := range ws.Users {
			obj.Subjects = append(obj.Subjects, rbacv1.Subject{
				Kind: "User",
				Name: fmt.Sprintf("id:%s", user),
			})
		}
	}

	return obj
}
