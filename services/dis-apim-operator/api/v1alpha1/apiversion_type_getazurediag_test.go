package v1alpha1

import (
	"testing"

	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGetAzureAPIAppInsightsDiagnosticSettings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ApiVersion Suite")
}

var _ = Describe("ApiVersion", func() {
	var (
		apiVersion      ApiVersion
		defaultLoggerID string
	)
	BeforeEach(func() {
		defaultLoggerID = "defaultLogger"
		apiVersion = ApiVersion{
			Spec: ApiVersionSpec{
				ApiVersionSubSpec: ApiVersionSubSpec{
					Diagnostics: &ApiDiagnosticSpec{
						LoggerName:         &defaultLoggerID,
						SamplingPercentage: utils.ToPointer(int32(75)),
						EnableMetrics:      utils.ToPointer(false),
						Frontend: &PipelineDiagnosticSettings{
							Request: &HttpMessageDiagnostic{
								Headers: []*string{utils.ToPointer("Frontend-Request-Header")},
							},
							Response: &HttpMessageDiagnostic{
								Headers: []*string{utils.ToPointer("Frontend-Response-Header")},
							},
						},
						Backend: &PipelineDiagnosticSettings{
							Request: &HttpMessageDiagnostic{
								Headers: []*string{utils.ToPointer("Backend-Request-Header")},
							},
							Response: &HttpMessageDiagnostic{
								Headers: []*string{utils.ToPointer("Backend-Response-Header")},
							},
						},
					},
				},
			},
		}
	})

	Describe("GetAzureAPIDiagnosticSettings", func() {
		It("should return the specified diagnostic settings for the api", func() {
			diagnosticSettings := apiVersion.GetAzureAPIAppInsightsDiagnosticSettings(defaultLoggerID)

			Expect(diagnosticSettings.Properties.LoggerID).To(Equal(&defaultLoggerID))
			Expect(diagnosticSettings.Properties.Sampling.Percentage).To(Equal(utils.ToPointer(75.0)))
			Expect(diagnosticSettings.Properties.Metrics).To(Equal(utils.ToPointer(false)))
			Expect(diagnosticSettings.Properties.Frontend.Request.Headers).To(ContainElement(utils.ToPointer("Frontend-Request-Header")))
			Expect(diagnosticSettings.Properties.Frontend.Response.Headers).To(ContainElements(utils.ToPointer("Frontend-Response-Header")))
			Expect(diagnosticSettings.Properties.Backend.Request.Headers).To(ContainElements(utils.ToPointer("Backend-Request-Header")))
			Expect(diagnosticSettings.Properties.Backend.Response.Headers).To(ContainElement(utils.ToPointer("Backend-Response-Header")))
		})

		It("should not override frontend and backend headers when not specified", func() {
			apiVersion.Spec.Diagnostics.Frontend.Request.Headers = nil
			apiVersion.Spec.Diagnostics.Frontend.Response = nil
			apiVersion.Spec.Diagnostics.Backend.Request = nil
			apiVersion.Spec.Diagnostics.Backend.Response.Headers = nil
			diagnosticSettings := apiVersion.GetAzureAPIAppInsightsDiagnosticSettings(defaultLoggerID)

			Expect(diagnosticSettings.Properties.Frontend.Request.Headers).To(ContainElements(utils.ToPointer("Ocp-Apim-Subscription-Key"), utils.ToPointer("X-Forwarded-For"), utils.ToPointer("Content-Type")))
			Expect(diagnosticSettings.Properties.Frontend.Response.Headers).To(ContainElements(utils.ToPointer("Ocp-Apim-Subscription-Key"), utils.ToPointer("X-Forwarded-For"), utils.ToPointer("Content-Type")))
			Expect(diagnosticSettings.Properties.Backend.Request.Headers).To(ContainElements(utils.ToPointer("Ocp-Apim-Subscription-Key"), utils.ToPointer("X-Forwarded-For"), utils.ToPointer("Content-Type")))
			Expect(diagnosticSettings.Properties.Backend.Response.Headers).To(ContainElements(utils.ToPointer("Ocp-Apim-Subscription-Key"), utils.ToPointer("X-Forwarded-For"), utils.ToPointer("Content-Type")))
		})

		It("should use default values when diagnostics are not specified", func() {
			apiVersion.Spec.Diagnostics = nil
			diagnosticSettings := apiVersion.GetAzureAPIAppInsightsDiagnosticSettings(defaultLoggerID)

			Expect(diagnosticSettings.Properties.LoggerID).To(Equal(&defaultLoggerID))
			Expect(diagnosticSettings.Properties.Metrics).To(Equal(utils.ToPointer(true)))
			Expect(diagnosticSettings.Properties.AlwaysLog).To(Equal(utils.ToPointer(apim.AlwaysLogAllErrors)))
			Expect(diagnosticSettings.Properties.HTTPCorrelationProtocol).To(Equal(utils.ToPointer(apim.HTTPCorrelationProtocolW3C)))
			Expect(diagnosticSettings.Properties.Verbosity).To(Equal(utils.ToPointer(apim.VerbosityError)))
			Expect(diagnosticSettings.Properties.Sampling.Percentage).To(Equal(utils.ToPointer(50.0)))
			Expect(diagnosticSettings.Properties.Sampling.SamplingType).To(Equal(utils.ToPointer(apim.SamplingTypeFixed)))
			Expect(diagnosticSettings.Properties.Frontend.Request.Body.Bytes).To(Equal(utils.ToPointer(int32(0))))
			Expect(diagnosticSettings.Properties.Frontend.Request.Headers).To(ContainElements(utils.ToPointer("Ocp-Apim-Subscription-Key"), utils.ToPointer("X-Forwarded-For"), utils.ToPointer("Content-Type")))
			Expect(diagnosticSettings.Properties.Frontend.Response.Body.Bytes).To(Equal(utils.ToPointer(int32(0))))
			Expect(diagnosticSettings.Properties.Frontend.Response.Headers).To(ContainElements(utils.ToPointer("Ocp-Apim-Subscription-Key"), utils.ToPointer("X-Forwarded-For"), utils.ToPointer("Content-Type")))
			Expect(diagnosticSettings.Properties.Backend.Request.Body.Bytes).To(Equal(utils.ToPointer(int32(0))))
			Expect(diagnosticSettings.Properties.Backend.Request.Headers).To(ContainElements(utils.ToPointer("Ocp-Apim-Subscription-Key"), utils.ToPointer("X-Forwarded-For"), utils.ToPointer("Content-Type")))
			Expect(diagnosticSettings.Properties.Backend.Response.Body.Bytes).To(Equal(utils.ToPointer(int32(0))))
			Expect(diagnosticSettings.Properties.Backend.Response.Headers).To(ContainElements(utils.ToPointer("Ocp-Apim-Subscription-Key"), utils.ToPointer("X-Forwarded-For"), utils.ToPointer("Content-Type")))
		})
	})
})
