# CreateGlossaryTermRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**definition** | **str** |  | [optional] 
**description** | **str** |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | [optional] 
**parent** | **str** |  | [optional] 
**synonyms** | **List[str]** |  | [optional] 
**tags** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.create_glossary_term_request import CreateGlossaryTermRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CreateGlossaryTermRequest from a JSON string
create_glossary_term_request_instance = CreateGlossaryTermRequest.from_json(json)
# print the JSON string representation of the object
print(CreateGlossaryTermRequest.to_json())

# convert the object into a dict
create_glossary_term_request_dict = create_glossary_term_request_instance.to_dict()
# create an instance of CreateGlossaryTermRequest from a dict
create_glossary_term_request_from_dict = CreateGlossaryTermRequest.from_dict(create_glossary_term_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


