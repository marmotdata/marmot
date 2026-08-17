# SSOProvidersResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**providers** | [**List[SSOProvider]**](SSOProvider.md) |  | [optional] 

## Example

```python
from marmot.generated.models.sso_providers_response import SSOProvidersResponse

# TODO update the JSON string below
json = "{}"
# create an instance of SSOProvidersResponse from a JSON string
sso_providers_response_instance = SSOProvidersResponse.from_json(json)
# print the JSON string representation of the object
print(SSOProvidersResponse.to_json())

# convert the object into a dict
sso_providers_response_dict = sso_providers_response_instance.to_dict()
# create an instance of SSOProvidersResponse from a dict
sso_providers_response_from_dict = SSOProvidersResponse.from_dict(sso_providers_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


