# DataProductOwnerRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **str** |  | 
**type** | **str** |  | 

## Example

```python
from marmot.generated.models.data_product_owner_request import DataProductOwnerRequest

# TODO update the JSON string below
json = "{}"
# create an instance of DataProductOwnerRequest from a JSON string
data_product_owner_request_instance = DataProductOwnerRequest.from_json(json)
# print the JSON string representation of the object
print(DataProductOwnerRequest.to_json())

# convert the object into a dict
data_product_owner_request_dict = data_product_owner_request_instance.to_dict()
# create an instance of DataProductOwnerRequest from a dict
data_product_owner_request_from_dict = DataProductOwnerRequest.from_dict(data_product_owner_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


