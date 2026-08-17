# ServiceAccountAPIKey


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **str** |  | [optional] 
**expires_at** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**key** | **str** |  | [optional] 
**last_used_at** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**service_account_id** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.service_account_api_key import ServiceAccountAPIKey

# TODO update the JSON string below
json = "{}"
# create an instance of ServiceAccountAPIKey from a JSON string
service_account_api_key_instance = ServiceAccountAPIKey.from_json(json)
# print the JSON string representation of the object
print(ServiceAccountAPIKey.to_json())

# convert the object into a dict
service_account_api_key_dict = service_account_api_key_instance.to_dict()
# create an instance of ServiceAccountAPIKey from a dict
service_account_api_key_from_dict = ServiceAccountAPIKey.from_dict(service_account_api_key_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


