# Style and conventions

## One sentence per line

To keep consistency throughout the Markdown files in the Open Container spec all files should be formatted one sentence per line.
This fixes two things: it makes diffing easier with git and it resolves fights about line wrapping length.
For example, this paragraph will span three lines in the Markdown source.

## Traditionally hex settings should use JSON integers, not JSON strings

For example, [`"classID": 1048577`][class-id] instead of `"classID": "0x100001"`.
The config JSON isn't enough of a UI to be worth jumping through string <-> integer hoops to support an 0x… form ([source][integer-over-hex]).

## Constant names should keep redundant prefixes

For example, `CAP_KILL` instead of `KILL` in [**`linux.capabilities`**][capabilities].
The redundancy reduction from removing the namespacing prefix is not useful enough to be worth trimming the upstream identifier ([source][keep-prefix]).

## Optional settings should have pointer Go types

So we have a consistent way to identify unset values ([source][optional-pointer]).
The exceptions are entries where the Go default for the type is a no-op in the spec, in which case `omitempty` is sufficient and no pointer is needed (sources [here][no-pointer-for-slices], [here][no-pointer-for-boolean], and [here][pointer-when-updates-require-changes]).

## Examples

### Anchoring

For any given section that provides a notable example, it is ideal to have it denoted with [markdown headers][markdown-headers].
The level of header should be such that it is a subheader of the header it is an example of.

#### Example

```markdown
## Some Topic

### Some Subheader

#### Further Subheader

##### Example

To use Further Subheader, ...

### Example

To use Some Topic, ...

```

### Content

Where necessary, the values in the example can be empty or unset, but accommodate with comments regarding this intention.

Where feasible, the content and values used in an example should convey the fullest use of the data structures concerned.
Most commonly onlookers will intend to copy-and-paste a "working example".
If the intention of the example is to be a fully utilized example, rather than a copy-and-paste example, perhaps add a comment as such.

```markdown
### Example
```
```json
{
    "foo": null,
    "bar": ""
}
```

**vs.**

```markdown
### Example

Following is a fully populated example (not necessarily for copy/paste use)
```
```json
{
    "foo": [
        1,
        2,
        3
    ],
    "bar": "waffles",
    "bif": {
        "baz": "potatoes"
    }
}
```

[capabilities]: config-linux.md#capabilities
[class-id]: config-linux.md#network
[integer-over-hex]: https://github.com/opencontainers/runtime-spec/pull/267#discussion_r48360013
[keep-prefix]: https://github.com/opencontainers/runtime-spec/pull/159#issuecomment-138728337
[no-pointer-for-boolean]: https://github.com/opencontainers/runtime-spec/pull/290#discussion_r50296396
[no-pointer-for-slices]: https://github.com/opencontainers/runtime-spec/pull/316/files#r50782982
[optional-pointer]: https://github.com/opencontainers/runtime-spec/pull/233#discussion_r47829711
[pointer-when-updates-require-changes]: https://github.com/opencontainers/runtime-spec/pull/317/files#r50932706
[markdown-headers]: https://help.github.com/articles/basic-writing-and-formatting-syntax/#headings
