/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"time"

	v1 "github.com/labring/laf/core/controllers/oss/api/v1"
	"github.com/labring/laf/core/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gatewayv1 "github.com/labring/laf/core/controllers/gateway/api/v1"
)

const gatewayFinalizer = "gateway.gateway.laf.dev"

// GatewayReconciler reconciles a Gateway object
type GatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gateway.laf.dev,resources=gateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.laf.dev,resources=gateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.laf.dev,resources=gateways/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Gateway object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	// get gateway
	gateway := &gatewayv1.Gateway{}
	if err := r.Get(ctx, req.NamespacedName, gateway); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if gateway.DeletionTimestamp.IsZero() {
		return r.apply(ctx, gateway)
	}
	return ctrl.Result{}, nil
}

func (r *GatewayReconciler) apply(ctx context.Context, gateway *gatewayv1.Gateway) (ctrl.Result, error) {
	_log := log.FromContext(ctx)

	if gateway.Status.AppRoute == nil {
		result, err := r.applyApp(ctx, gateway)
		if err != nil {
			return result, err
		}
	}

	// apply bucket route
	result, err := r.applyBucket(ctx, gateway)
	if err != nil {
		return result, err
	}

	// update ready condition
	if util.ConditionIsTrue(gateway.Status.Conditions, "Ready") == false {
		condition := metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Reason:             "GatewayReady",
			Message:            "Gateway is ready",
		}

		util.SetCondition(&gateway.Status.Conditions, condition)
		if err := r.updateStatus(ctx, types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}, gateway.Status.DeepCopy()); err != nil {
			return ctrl.Result{}, err
		}
		_log.Info("Updated gateway condition to ready", "gateway", gateway.Name)
		return ctrl.Result{}, nil
	}

	_log.Info("apply gateway: name success", "name", gateway.Name)

	return ctrl.Result{}, nil
}

// applyDomain apply domain
func (r *GatewayReconciler) applyApp(ctx context.Context, gateway *gatewayv1.Gateway) (ctrl.Result, error) {
	_log := log.FromContext(ctx)

	// TODO select app from application
	region := "default"

	// select app domain
	appDomain, err := r.selectDomain(ctx, gatewayv1.APP, region)
	if err != nil {
		return ctrl.Result{}, err
	}
	if appDomain == nil {
		_log.Info("no app domain found")
		return ctrl.Result{}, errors.New("no app domain found")
	}

	// set gateway status
	routeStatus := &gatewayv1.GatewayRoute{
		DomainName:      appDomain.Name,
		DomainNamespace: appDomain.Namespace,
		Domain:          gateway.Spec.AppId + "." + appDomain.Spec.Domain,
	}

	// create app route
	appRoute := &gatewayv1.Route{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "app",
			Namespace: gateway.Namespace,
		},
		Spec: gatewayv1.RouteSpec{
			Domain:          routeStatus.Domain,
			DomainName:      appDomain.Name,
			DomainNamespace: appDomain.Namespace,
			Backend: gatewayv1.Backend{
				ServiceName: gateway.Spec.AppId + "." + gateway.Namespace,
				ServicePort: 8000,
			},
			EnableWebSocket: true,
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, appRoute, func() error {
		if err := controllerutil.SetControllerReference(gateway, appRoute, r.Scheme); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}

	// update gateway status
	gateway.Status.AppRoute = routeStatus

	if err = r.updateStatus(ctx, types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}, gateway.Status.DeepCopy()); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// applyBucket apply bucket
func (r *GatewayReconciler) applyBucket(ctx context.Context, gateway *gatewayv1.Gateway) (ctrl.Result, error) {

	// create bucket route
	createBuckets := make([]string, 0)
	for _, bucketName := range gateway.Spec.Buckets {
		if _, ok := gateway.Status.BucketRoutes[bucketName]; !ok {
			createBuckets = append(createBuckets, bucketName)
		}
	}
	if len(createBuckets) != 0 {
		result, err := r.addBuckets(ctx, gateway, createBuckets)
		if err != nil {
			return result, err
		}
	}

	// delete bucket route
	deleteBuckets := make([]string, 0)
	for bucketName := range gateway.Status.BucketRoutes {
		if !util.ContainsString(gateway.Spec.Buckets, bucketName) {
			deleteBuckets = append(deleteBuckets, bucketName)
		}
	}
	if len(deleteBuckets) != 0 {
		result, err := r.deleteBuckets(ctx, gateway, deleteBuckets)
		if err != nil {
			return result, err
		}
	}

	return ctrl.Result{}, nil
}

// addBuckets add buckets
func (r *GatewayReconciler) addBuckets(ctx context.Context, gateway *gatewayv1.Gateway, buckets []string) (ctrl.Result, error) {
	_log := log.FromContext(ctx)

	// if gateway status bucketRoutes is nil, init it
	if gateway.Status.BucketRoutes == nil {
		gateway.Status.BucketRoutes = make(map[string]*gatewayv1.GatewayRoute, 0)
	}

	// select bucket domain
	for _, bucketName := range buckets {

		// if bucket route is not exist, create it
		if _, ok := gateway.Status.BucketRoutes[bucketName]; ok {
			continue
		}

		// get bucket
		bucket := v1.Bucket{}
		err := r.Get(ctx, client.ObjectKey{Namespace: gateway.Namespace, Name: bucketName}, &bucket)
		if err != nil {
			return ctrl.Result{}, err
		}
		// get user
		user := v1.User{}
		err = r.Get(ctx, client.ObjectKey{Namespace: gateway.Namespace, Name: bucket.Status.User}, &user)
		if err != nil {
			return ctrl.Result{}, err
		}
		// get store
		store := v1.Store{}
		err = r.Get(ctx, client.ObjectKey{Namespace: user.Status.StoreNamespace, Name: user.Status.StoreName}, &store)
		if err != nil {
			return ctrl.Result{}, err
		}
		// select bucket domain
		bucketDomain, err := r.selectDomain(ctx, gatewayv1.BUCKET, store.Spec.Region)
		if err != nil {
			return ctrl.Result{}, err
		}
		if bucketDomain == nil {
			_log.Info("no bucket domain found")
			continue
		}

		routeStatus := &gatewayv1.GatewayRoute{
			DomainName:      bucketDomain.Name,
			DomainNamespace: bucketDomain.Namespace,
			Domain:          bucketName + "." + bucketDomain.Spec.Domain,
		}

		// create bucket route
		bucketRoute := &gatewayv1.Route{
			ObjectMeta: ctrl.ObjectMeta{
				Name:      "bucket-" + bucketName,
				Namespace: gateway.Namespace,
			},
			Spec: gatewayv1.RouteSpec{
				Domain:          routeStatus.Domain,
				DomainName:      bucketDomain.Name,
				DomainNamespace: bucketDomain.Namespace,
				Backend: gatewayv1.Backend{
					ServiceName: user.Status.Endpoint,
					ServicePort: 0, // If set to 0, the port is not used
				},
				PassHost: bucketName + "." + user.Status.Endpoint,
			},
		}

		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, bucketRoute, func() error {
			if err := controllerutil.SetControllerReference(gateway, bucketRoute, r.Scheme); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}
		gateway.Status.BucketRoutes[bucketName] = routeStatus

		if err = r.updateStatus(ctx, types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}, gateway.Status.DeepCopy()); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// deleteBuckets delete buckets
func (r *GatewayReconciler) deleteBuckets(ctx context.Context, gateway *gatewayv1.Gateway, buckets []string) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// find deleted bucket, remote route and finalizer
	for _, bucketName := range buckets {
		// delete route
		route := &gatewayv1.Route{}

		// 删除名称为test的route
		if err := r.Get(ctx, client.ObjectKey{Namespace: gateway.Namespace, Name: "bucket-" + bucketName}, route); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return ctrl.Result{}, err
		}

		if err := r.Delete(ctx, route); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return ctrl.Result{}, err
		}

		// delete bucket route
		delete(gateway.Status.BucketRoutes, bucketName)
		if err := r.updateStatus(ctx, types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}, gateway.Status.DeepCopy()); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *GatewayReconciler) selectDomain(ctx context.Context, backendType gatewayv1.BackendType, region string) (*gatewayv1.Domain, error) {
	_ = log.FromContext(ctx)

	// get all domains
	var domains gatewayv1.DomainList
	if err := r.List(ctx, &domains); err != nil {
		return nil, err
	}

	// select domain
	for _, domain := range domains.Items {
		if domain.Spec.BackendType != backendType {
			continue
		}

		if domain.Spec.Region != region {
			continue
		}
		return &domain, nil
	}
	return nil, nil
}

func (r *GatewayReconciler) updateStatus(ctx context.Context, nn types.NamespacedName, status *gatewayv1.GatewayStatus) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		original := &gatewayv1.Gateway{}
		if err := r.Get(ctx, nn, original); err != nil {
			return err
		}
		original.Status = *status
		if err := r.Client.Status().Update(ctx, original); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.Gateway{}).
		Complete(r)
}
