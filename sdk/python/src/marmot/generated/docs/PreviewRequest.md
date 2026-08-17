# PreviewRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**limit** | **int** |  | [optional] 
**metadata_field** | **str** |  | [optional] 
**pattern_type** | **str** |  | [optional] 
**pattern_value** | **str** |  | [optional] 
**query_expression** | **str** |  | [optional] 
**rule_type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.preview_request import PreviewRequest

# TODO update the JSON string below
json = "{}"
# create an instance of PreviewRequest from a JSON string
preview_request_instance = PreviewRequest.from_json(json)
# print the JSON string representation of the object
print(PreviewRequest.to_json())

# convert the object into a dict
preview_request_dict = preview_request_instance.to_dict()
# create an instance of PreviewRequest from a dict
preview_request_from_dict = PreviewRequest.from_dict(preview_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


