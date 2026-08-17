# DataProductOwner


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**email** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**profile_picture** | **str** |  | [optional] 
**type** | **str** |  | [optional] 
**username** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.data_product_owner import DataProductOwner

# TODO update the JSON string below
json = "{}"
# create an instance of DataProductOwner from a JSON string
data_product_owner_instance = DataProductOwner.from_json(json)
# print the JSON string representation of the object
print(DataProductOwner.to_json())

# convert the object into a dict
data_product_owner_dict = data_product_owner_instance.to_dict()
# create an instance of DataProductOwner from a dict
data_product_owner_from_dict = DataProductOwner.from_dict(data_product_owner_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


