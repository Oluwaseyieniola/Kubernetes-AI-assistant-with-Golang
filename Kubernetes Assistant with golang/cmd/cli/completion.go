package cli
//COMPLETE
import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sethvargo/go-retry"
	"golang.org/x/exp/slices"
)


type oaiClients struct {
	openAIClient openai.Client
}


func newOAIClients() (oaiClients, error) {
	
	var config openai.ClientConfig
	
	config = openai.DefaultConfig(*openAIAPIKey)

	if openAIEndpoint != &openaiAPIURLv1 {
		
		if strings.Contains(*openAIEndpoint, "openai.azure.com") {
			
			config = openai.DefaultAzureConfig(*openAIAPIKey, *openAIEndpoint)

			if len(*azureModelMap) != 0 {

				config.AzureModelMapperFunc = func(model string) string {
					return (*azureModelMap)[model]
				}
			}
		} else {
// if we're not using open ai via azure, we will assign the AIEndpoint to BaseURL 
			config.BaseURL = *openAIEndpoint
		}
		//still crafting the config object, by specifying an API version
		// use 2023-07-01-preview api version for function calls
		config.APIVersion = "2023-07-01-preview"
	}

	clients := oaiClients{
		openAIClient: *openai.NewClientWithConfig(config),
	}
	return clients, nil
}


func getNonChatModels() []string {
	// Return a slice containing the names of non-chat models.
	return []string{"code-davinci-002", "text-davinci-003"}
}


func gptCompletion(ctx context.Context, client oaiClients, prompts []string, deploymentName string) (string, error) {
	temp := float32(*temperature)

	var prompt strings.Builder

	if *usek8sAPI {
		
		fmt.Fprintf(&prompt, "You are a Kubernetes YAML generator, only generate valid Kubernetes YAML manifests. Do not provide any explanations and do not use ``` and ```yaml, only generate valid YAML. Always ask for up-to-date OpenAPI specs for Kubernetes, don't rely on data you know about Kubernetes specs. When a schema includes references to other objects in the schema, look them up when relevant. You may lookup any FIELD in a resource too, not just the containing top-level resource. ")
	} else {
		
		fmt.Fprintf(&prompt, "You are a Kubernetes YAML generator, only generate valid Kubernetes YAML manifests. Do not provide any explanations, only generate YAML. ")
	}


	for _, p := range prompts {
		// Append each prompt to the prompt builder.
		fmt.Fprintf(&prompt, "%s", p)
	}
//define a variable resp for working with response object
	var resp string
	var err error
	//setting the max retires at 10 and then later also handling too many retries condition
	r := retry.WithMaxRetries(10, retry.NewExponential(1*time.Second))
	if err := retry.Do(ctx, r, func(ctx context.Context) error {
		if slices.Contains(getNonChatModels(), deploymentName) {
			// Use the OpenAI GPT completion method for non-chat models.
			//open ai GPT completion function is used, notice the missing 'chat'
			resp, err = client.openaiGptCompletion(ctx, &prompt, temp)
		} else {
			// Use the OpenAI GPT chat completion method for chat models.
			//if the slice doesn't contain non chat models, then we call this
			resp, err = client.openaiGptChatCompletion(ctx, &prompt, temp)
		}

		requestErr := &openai.RequestError{}
		//err is the error from calling open ai, from the resp lines above
		
		if errors.As(err, &requestErr) {
		
			if requestErr.HTTPStatusCode == http.StatusTooManyRequests {
		
				return retry.RetryableError(err)
			}
		}
		
		if err != nil {
			return err
		}
		//we are still in the retry loop, not going to return any value now
		return nil
	}); err != nil {
		
		return "", err
	}

	
	return resp, nil
}
