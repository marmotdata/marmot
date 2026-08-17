# CreateServiceAccountAPIKeyRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**expires_in_days** | **int** |  | [optional] 
**name** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.create_service_account_api_key_request import CreateServiceAccountAPIKeyRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CreateServiceAccountAPIKeyRequest from a JSON string
create_service_account_api_key_request_instance = CreateServiceAccountAPIKeyRequest.from_json(json)
# print the JSON string representation of the object
print(CreateServiceAccountAPIKeyRequest.to_json())

# convert the object into a dict
create_service_account_api_key_request_dict = create_service_account_api_key_request_instance.to_dict()
# create an instance of CreateServiceAccountAPIKeyRequest from a dict
create_service_account_api_key_request_from_dict = CreateServiceAccountAPIKeyRequest.from_dict(create_service_account_api_key_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


