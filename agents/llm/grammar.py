"""GBNF grammar for the focal agent's action output.

Covers the FULL social-emergence verb set (D1-D22), unlike the older
examples/qwen_agent grammar which predates eat/attack/equip/trade/
propose_task. Grammar-constrained decoding guarantees the model emits
schema-valid JSON even with reasoning-budget=0 (per the local-Qwen
reference setup), so we never have to defensively parse malformed
output.

Output shape:
    {"reasoning": "<one short sentence>",
     "actions": [ <1-3 action objects> ]}

Action targets that reference another agent or item MUST be the
entity_id from the observation (D1), never a display name.
"""

FOCAL_GRAMMAR = r"""
root ::= "{" ws "\"reasoning\":" ws string "," ws "\"actions\":" ws action-list ws "}"

action-list ::= "[" ws action ws ("," ws action ws)* "]"

action ::= goto-action | pursue-action | flee-action | step-action
         | speak-action | whisper-action | shout-action
         | eat-action | pickup-action | equip-action | give-action
         | pay-action | buyfood-action | trade-action | attack-action
         | propose-action | accept-action | complete-action | reject-action
         | enter-action | exit-action | wait-action

goto-action    ::= "{" ws "\"verb\":" ws "\"goto\"" "," ws "\"target\":" ws "[" ws integer ws "," ws integer ws "]" ws "}"
pursue-action  ::= "{" ws "\"verb\":" ws "\"pursue\"" "," ws "\"target\":" ws string ws "}"
flee-action    ::= "{" ws "\"verb\":" ws "\"flee\"" "," ws "\"target\":" ws string ws "}"
step-action    ::= "{" ws "\"verb\":" ws "\"step\"" "," ws "\"dir\":" ws direction ws "}"
direction      ::= "\"N\"" | "\"S\"" | "\"E\"" | "\"W\""
speak-action   ::= "{" ws "\"verb\":" ws "\"speak\"" "," ws "\"text\":" ws string ws "}"
whisper-action ::= "{" ws "\"verb\":" ws "\"whisper\"" "," ws "\"target\":" ws string "," ws "\"text\":" ws string ws "}"
shout-action   ::= "{" ws "\"verb\":" ws "\"shout\"" "," ws "\"text\":" ws string ws "}"
eat-action     ::= "{" ws "\"verb\":" ws "\"eat\"" "," ws "\"item\":" ws string ws "}"
pickup-action  ::= "{" ws "\"verb\":" ws "\"pickup\"" "," ws "\"target\":" ws string ws "}"
equip-action   ::= "{" ws "\"verb\":" ws "\"equip\"" "," ws "\"item\":" ws string ("," ws "\"slot\":" ws string)? ws "}"
give-action    ::= "{" ws "\"verb\":" ws "\"give\"" "," ws "\"target\":" ws string "," ws "\"item\":" ws string ws "}"
pay-action     ::= "{" ws "\"verb\":" ws "\"pay\"" "," ws "\"target\":" ws string "," ws "\"amount\":" ws integer ws "}"
buyfood-action ::= "{" ws "\"verb\":" ws "\"buy_food\"" ws "}"
trade-action   ::= "{" ws "\"verb\":" ws "\"trade\"" "," ws "\"target\":" ws string "," ws "\"item\":" ws string "," ws "\"price\":" ws integer ws "}"
attack-action  ::= "{" ws "\"verb\":" ws "\"attack\"" "," ws "\"target\":" ws string ws "}"
propose-action ::= "{" ws "\"verb\":" ws "\"propose_task\"" "," ws "\"target\":" ws string "," ws "\"terms\":" ws string ("," ws "\"reward\":" ws string)? ws "}"
accept-action  ::= "{" ws "\"verb\":" ws "\"accept_task\"" "," ws "\"id\":" ws string ws "}"
complete-action ::= "{" ws "\"verb\":" ws "\"complete_task\"" "," ws "\"id\":" ws string ws "}"
reject-action  ::= "{" ws "\"verb\":" ws "\"reject_task\"" "," ws "\"id\":" ws string ws "}"
enter-action   ::= "{" ws "\"verb\":" ws "\"enter\"" "," ws "\"target\":" ws string ws "}"
exit-action    ::= "{" ws "\"verb\":" ws "\"exit\"" ws "}"
wait-action    ::= "{" ws "\"verb\":" ws "\"wait\"" ("," ws "\"ticks\":" ws integer)? ws "}"

string  ::= "\"" char* "\""
char    ::= [a-zA-Z0-9 .,!?'#:_-]
integer ::= "-"? [0-9]+
ws      ::= [ \t\n]*
"""
