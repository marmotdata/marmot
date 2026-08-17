# BatchCreateRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**assets** | [**List[RunCreateAssetRequest]**](RunCreateAssetRequest.md) |  | 
**config** | **Dict[str, object]** |  | [optional] 
**documentation** | [**List[CreateDocRequest]**](CreateDocRequest.md) |  | [optional] 
**glossary_terms** | [**List[CreateGlossaryTermRequest]**](CreateGlossaryTermRequest.md) |  | [optional] 
**lineage** | [**List[CreateLineageRequest]**](CreateLineageRequest.md) |  | [optional] 
**pipeline_name** | **str** |  | 
**run_history** | [**List[CreateRunHistoryRequest]**](CreateRunHistoryRequest.md) |  | [optional] 
**run_id** | **str** |  | 
**source_name** | **str** |  | 
**statistics** | [**List[CreateStatRequest]**](CreateStatRequest.md) |  | [optional] 

## Example

```python
from marmot.generated.models.batch_create_request import BatchCreateRequest

# TODO update the JSON string below
json = "{}"
# create an instance of BatchCreateRequest from a JSON string
batch_create_request_instance = BatchCreateRequest.from_json(json)
# print the JSON string representation of the object
print(BatchCreateRequest.to_json())

# convert the object into a dict
batch_create_request_dict = batch_create_request_instance.to_dict()
# create an instance of BatchCreateRequest from a dict
batch_create_request_from_dict = BatchCreateRequest.from_dict(batch_create_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


