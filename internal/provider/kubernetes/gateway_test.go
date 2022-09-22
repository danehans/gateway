package kubernetes

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gwapiv1b1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/envoyproxy/gateway/api/config/v1alpha1"
	"github.com/envoyproxy/gateway/internal/envoygateway"
	"github.com/envoyproxy/gateway/internal/log"
)

func TestGatewayHasMatchingController(t *testing.T) {
	match := &gwapiv1b1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "matched",
		},
		Spec: gwapiv1b1.GatewayClassSpec{
			ControllerName: v1alpha1.GatewayControllerName,
		},
	}

	nonMatch := &gwapiv1b1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-matched",
		},
		Spec: gwapiv1b1.GatewayClassSpec{
			ControllerName: "not.configured/controller-name",
		},
	}

	testCases := []struct {
		name   string
		obj    client.Object
		expect bool
	}{
		{
			name: "matching object type, gatewayclass, and controller name",
			obj: &gwapiv1b1.Gateway{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Gateway",
					APIVersion: fmt.Sprintf("%s/%s", gwapiv1b1.GroupName, gwapiv1b1.GroupVersion.Version),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: gwapiv1b1.GatewaySpec{
					GatewayClassName: gwapiv1b1.ObjectName(match.Name),
				},
			},
			expect: true,
		},
		{
			name: "matching object type but gatewayclass doesn't exist",
			obj: &gwapiv1b1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: gwapiv1b1.GatewaySpec{
					GatewayClassName: "non-existent-gc",
				},
			},
			expect: false,
		},
		{
			name: "matching object type and gatewayclass but not controller name",
			obj: &gwapiv1b1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: gwapiv1b1.GatewaySpec{
					GatewayClassName: gwapiv1b1.ObjectName(nonMatch.Name),
				},
			},
			expect: false,
		},
		{
			name: "gatewayclass name match but object type doesn't match",
			obj: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: gwapiv1b1.GatewayController(v1alpha1.GatewayControllerName),
				},
			},
			expect: false,
		},
	}

	// Create the reconciler.
	logger, err := log.NewLogger()
	require.NoError(t, err)
	r := gatewayReconciler{
		classController: v1alpha1.GatewayControllerName,
		log:             logger,
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r.client = fakeclient.NewClientBuilder().WithScheme(envoygateway.GetScheme()).WithObjects(match, nonMatch, tc.obj).Build()
			actual := r.gatewayHasMatchingController(tc.obj)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestClassHasMatchingController(t *testing.T) {
	testCases := []struct {
		name     string
		obj      client.Object
		gateways []gwapiv1b1.Gateway
		expect   bool
	}{
		{
			name: "matching object type, controller name, and managed gateway",
			obj: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gc1",
					Namespace: "test",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: v1alpha1.GatewayControllerName,
				},
			},
			gateways: []gwapiv1b1.Gateway{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Gateway",
						APIVersion: fmt.Sprintf("%s/%s", gwapiv1b1.GroupName, gwapiv1b1.GroupVersion.Version),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName("gc1"),
					},
				},
			},
			expect: true,
		},
		{
			name: "matching object type and controller name, but no managed gateways",
			obj: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gc1",
					Namespace: "test",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: v1alpha1.GatewayControllerName,
				},
			},
			gateways: []gwapiv1b1.Gateway{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Gateway",
						APIVersion: fmt.Sprintf("%s/%s", gwapiv1b1.GroupName, gwapiv1b1.GroupVersion.Version),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName("unmanaged-gc"),
					},
				},
			},
			expect: false,
		},
		{
			name: "matching object type and gatewayclass but not controller name",
			obj: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gc1",
					Namespace: "test",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: "not.envoy.gateway.controller",
				},
			},
			expect: false,
		},
		{
			name: "not gatewayclass object type",
			obj: &gwapiv1b1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gw1",
					Namespace: "test",
				},
			},
			expect: false,
		},
	}

	// Create the reconciler.
	logger, err := log.NewLogger()
	require.NoError(t, err)
	r := gatewayReconciler{
		classController: v1alpha1.GatewayControllerName,
		log:             logger,
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var objs []client.Object
			for i := range tc.gateways {
				objs = append(objs, &tc.gateways[i])
			}
			r.client = fakeclient.NewClientBuilder().WithScheme(envoygateway.GetScheme()).WithObjects(objs...).Build()
			actual := r.classHasMatchingController(tc.obj)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestGetGatewaysForClass(t *testing.T) {
	testCases := []struct {
		name     string
		obj      client.Object
		gateways []gwapiv1b1.Gateway
		expect   []reconcile.Request
	}{
		{
			name: "one gateway matches gatewayclass",
			obj: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "gc1",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: "test.controller",
				},
			},
			gateways: []gwapiv1b1.Gateway{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "gw1",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName("gc1"),
					},
				},
			},
			expect: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: "test",
						Name:      "gw1",
					},
				},
			},
		},
		{
			name: "one of two gateways match gatewayclass",
			obj: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "gc1",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: "test.controller",
				},
			},
			gateways: []gwapiv1b1.Gateway{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "gw1",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName("gc1"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "gw2",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName("gc2"),
					},
				},
			},
			expect: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: "test",
						Name:      "gw1",
					},
				},
			},
		},
		{
			name: "not a gatewayclass object",
			obj: &gwapiv1b1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "gw1",
				},
			},
			expect: []reconcile.Request{},
		},
	}

	// Create the reconciler.
	logger, err := log.NewLogger()
	require.NoError(t, err)
	r := &gatewayReconciler{
		log:             logger,
		classController: gwapiv1b1.GatewayController(v1alpha1.GatewayControllerName),
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			objs := []client.Object{tc.obj}
			for i := range tc.gateways {
				objs = append(objs, &tc.gateways[i])
			}
			r.client = fakeclient.NewClientBuilder().WithScheme(envoygateway.GetScheme()).WithObjects(objs...).Build()
			reqs := r.getGatewaysForClass(tc.obj)
			assert.Equal(t, tc.expect, reqs)
		})
	}
}

func TestIsAccepted(t *testing.T) {
	testCases := []struct {
		name   string
		gc     *gwapiv1b1.GatewayClass
		expect bool
	}{
		{
			name: "gatewayclass accepted condition",
			gc: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: gwapiv1b1.GatewayController(v1alpha1.GatewayControllerName),
				},
				Status: gwapiv1b1.GatewayClassStatus{
					Conditions: []metav1.Condition{
						{
							Type:   string(gwapiv1b1.GatewayClassConditionStatusAccepted),
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expect: true,
		},
		{
			name: "gatewayclass not accepted condition",
			gc: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: gwapiv1b1.GatewayController(v1alpha1.GatewayControllerName),
				},
				Status: gwapiv1b1.GatewayClassStatus{
					Conditions: []metav1.Condition{
						{
							Type:   string(gwapiv1b1.GatewayClassConditionStatusAccepted),
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			expect: false,
		},
		{
			name: "no gatewayclass accepted condition type",
			gc: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: gwapiv1b1.GatewayController(v1alpha1.GatewayControllerName),
				},
				Status: gwapiv1b1.GatewayClassStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "SomeOtherType",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expect: false,
		},
		{
			name:   "nil gatewayclass",
			expect: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actual := isAccepted(tc.gc)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestGatewaysOfClass(t *testing.T) {
	gc := &gwapiv1b1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	testCases := []struct {
		name   string
		gws    []gwapiv1b1.Gateway
		expect int
	}{
		{
			name: "no matching gateways",
			gws: []gwapiv1b1.Gateway{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName("no-match"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName("no-match2"),
					},
				},
			},
			expect: 0,
		},
		{
			name: "one of two matching gateways",
			gws: []gwapiv1b1.Gateway{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName(gc.Name),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test2",
						Namespace: "test",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName("no-match"),
					},
				},
			},
			expect: 1,
		},
		{
			name: "two of two matching gateways",
			gws: []gwapiv1b1.Gateway{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName(gc.Name),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test2",
						Namespace: "test",
					},
					Spec: gwapiv1b1.GatewaySpec{
						GatewayClassName: gwapiv1b1.ObjectName(gc.Name),
					},
				},
			},
			expect: 2,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gwList := &gwapiv1b1.GatewayList{Items: tc.gws}
			actual := gatewaysOfClass(gc, gwList)
			require.Equal(t, tc.expect, len(actual))
		})
	}
}

func TestAddFinalizer(t *testing.T) {
	testCases := []struct {
		name   string
		gc     *gwapiv1b1.GatewayClass
		expect []string
	}{
		{
			name: "gatewayclass with no finalizers",
			gc: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-gc",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: v1alpha1.GatewayControllerName,
				},
			},
			expect: []string{gatewayClassFinalizer},
		},
		{
			name: "gatewayclass with a different finalizer",
			gc: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-gc",
					Finalizers: []string{"fooFinalizer"},
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: v1alpha1.GatewayControllerName,
				},
			},
			expect: []string{"fooFinalizer", gatewayClassFinalizer},
		},
		{
			name: "gatewayclass with existing gatewayclass finalizer",
			gc: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-gc",
					Finalizers: []string{gatewayClassFinalizer},
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: v1alpha1.GatewayControllerName,
				},
			},
			expect: []string{gatewayClassFinalizer},
		},
	}

	// Create the reconciler.
	r := new(gatewayReconciler)
	ctx := context.Background()

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r.client = fakeclient.NewClientBuilder().WithScheme(envoygateway.GetScheme()).WithObjects(tc.gc).Build()
			err := r.addFinalizer(ctx, tc.gc)
			require.NoError(t, err)
			key := types.NamespacedName{Name: tc.gc.Name}
			err = r.client.Get(ctx, key, tc.gc)
			require.NoError(t, err)
			require.Equal(t, tc.expect, tc.gc.Finalizers)
		})
	}
}

func TestRemoveFinalizer(t *testing.T) {
	testCases := []struct {
		name   string
		gc     *gwapiv1b1.GatewayClass
		expect []string
	}{
		{
			name: "gatewayclass with no finalizers",
			gc: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-gc",
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: v1alpha1.GatewayControllerName,
				},
			},
			expect: nil,
		},
		{
			name: "gatewayclass with a different finalizer",
			gc: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-gc",
					Finalizers: []string{"fooFinalizer"},
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: v1alpha1.GatewayControllerName,
				},
			},
			expect: []string{"fooFinalizer"},
		},
		{
			name: "gatewayclass with existing gatewayclass finalizer",
			gc: &gwapiv1b1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-gc",
					Finalizers: []string{gatewayClassFinalizer},
				},
				Spec: gwapiv1b1.GatewayClassSpec{
					ControllerName: v1alpha1.GatewayControllerName,
				},
			},
			expect: nil,
		},
	}

	// Create the reconciler.
	r := new(gatewayReconciler)
	ctx := context.Background()

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r.client = fakeclient.NewClientBuilder().WithScheme(envoygateway.GetScheme()).WithObjects(tc.gc).Build()
			err := r.removeFinalizer(ctx, tc.gc)
			require.NoError(t, err)
			key := types.NamespacedName{Name: tc.gc.Name}
			err = r.client.Get(ctx, key, tc.gc)
			require.NoError(t, err)
			require.Equal(t, tc.expect, tc.gc.Finalizers)
		})
	}
}
