package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +protobuf=true

// Breakglass is the Schema for the breakglasses API
type Breakglass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BreakglassSpec   `json:"spec,omitempty"`
	Status BreakglassStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +protobuf=true

// BreakglassList contains a list of Breakglass requests.
type BreakglassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Breakglass `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func init() {
	SchemeBuilder.Register(&Breakglass{}, &BreakglassList{})
}
