/*
Copyright 2025.

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

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PodWatcherReconciler reconciles a PodWatcher object
type PodWatcherReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodWatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	pod := &corev1.Pod{}
	err := r.Get(ctx, req.NamespacedName, pod)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get Pod")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling Pod", "namespace", pod.Namespace, "name", pod.Name, "labels", pod.Labels)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodWatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if pod, ok := e.Object.(*corev1.Pod); ok {
				if _, exists := pod.Labels["ecs.takutakahashi.dev/watch"]; exists {
					logf.Log.Info("Pod with watch label created", "namespace", pod.Namespace, "name", pod.Name)
					return true
				}
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if pod, ok := e.ObjectNew.(*corev1.Pod); ok {
				if _, exists := pod.Labels["ecs.takutakahashi.dev/watch"]; exists {
					return true
				}
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if pod, ok := e.Object.(*corev1.Pod); ok {
				if _, exists := pod.Labels["ecs.takutakahashi.dev/watch"]; exists {
					logf.Log.Info("Pod with watch label deleted", "namespace", pod.Namespace, "name", pod.Name)
					return true
				}
			}
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}, builder.WithPredicates(labelPredicate)).
		Named("podwatcher").
		Complete(r)
}
