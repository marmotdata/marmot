# CreateDataProductRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**description** | **str** |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | 
**owners** | [**List[DataProductOwnerRequest]**](DataProductOwnerRequest.md) |  | [optional] 
**rules** | [**List[DataProductRuleRequest]**](DataProductRuleRequest.md) |  | [optional] 
**tags** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.create_data_product_request import CreateDataProductRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CreateDataProductRequest from a JSON string
create_data_product_request_instance = CreateDataProductRequest.from_json(json)
# print the JSON string representation of the object
print(CreateDataProductRequest.to_json())

# convert the object into a dict
create_data_product_request_dict = create_data_product_request_instance.to_dict()
# create an instance of CreateDataProductRequest from a dict
create_data_product_request_from_dict = CreateDataProductRequest.from_dict(create_data_product_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


