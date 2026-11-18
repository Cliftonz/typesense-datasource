import React, { useEffect, useState } from 'react';
import { CodeEditor, Combobox, InlineField, InlineSwitch, Input, RadioButtonGroup, Stack } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from '../datasource';
import { BucketMode, EditorMode, Format, MyDataSourceOptions, MyQuery } from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

type TextKey =
  'q' | 'queryBy' | 'filterBy' | 'sortBy' | 'facetBy' | 'groupBy' | 'timeField' | 'valueField' | 'minInterval';

type NumberKey = 'perPage' | 'maxPages';

const isWholeInRange = (value: string, max: number) =>
  Number.isInteger(Number(value)) && Number(value) >= 1 && Number(value) <= max;

const withNumber = (query: MyQuery, key: NumberKey, value: string): MyQuery => {
  const { [key]: _cleared, ...rest } = query;
  return value === '' ? rest : { ...rest, [key]: Number(value) };
};

interface CollectionsResponse {
  canList: boolean;
  collections: string[];
}

const builderFields: Array<[TextKey, string]> = [
  ['q', 'Query'],
  ['queryBy', 'Query by'],
  ['filterBy', 'Filter by'],
  ['sortBy', 'Sort by'],
  ['facetBy', 'Facet by'],
  ['groupBy', 'Group by'],
];

const editorModes = [
  { label: 'Builder', value: 'builder' as const },
  { label: 'Raw', value: 'raw' as const },
];

const formats = [
  { label: 'Table', value: 'table' as const },
  { label: 'Time series', value: 'timeseries' as const },
];

const bucketModes = [
  { label: 'Hits', value: 'hits' as const },
  { label: 'Range facet', value: 'rangeFacet' as const },
  { label: 'Group by', value: 'groupBy' as const },
];

export function QueryEditor({ datasource, query, onChange, onRunQuery }: Props) {
  const [listing, setListing] = useState<CollectionsResponse>();

  useEffect(() => {
    datasource
      .getResource<CollectionsResponse>('collections')
      .then(setListing)
      .catch(() => {});
  }, [datasource]);

  const changeAndRun = (next: MyQuery) => {
    onChange(next);
    onRunQuery();
  };

  const textField = (key: TextKey, label: string) => (
    <InlineField key={key} label={label} labelWidth={16}>
      <Input
        id={`query-editor-${key}`}
        value={query[key] ?? ''}
        onChange={(e) => onChange({ ...query, [key]: e.currentTarget.value })}
        onBlur={onRunQuery}
      />
    </InlineField>
  );

  const numberField = (key: NumberKey, label: string, max?: number) => (
    <InlineField key={key} label={label} labelWidth={16}>
      <Input
        id={`query-editor-${key}`}
        type="number"
        min={1}
        max={max}
        step={1}
        value={query[key] ?? ''}
        onChange={(e) => {
          const value = e.currentTarget.value;
          if (value === '' || isWholeInRange(value, max ?? Infinity)) {
            onChange(withNumber(query, key, value));
          }
        }}
        onBlur={onRunQuery}
      />
    </InlineField>
  );

  return (
    <Stack direction="column" gap={0}>
      <RadioButtonGroup<EditorMode>
        options={editorModes}
        value={query.editorMode}
        onChange={(editorMode) => changeAndRun({ ...query, editorMode })}
      />
      {query.editorMode === 'raw' ? (
        <CodeEditor
          language="json"
          height={200}
          value={query.rawBody ?? ''}
          onBlur={(rawBody) => changeAndRun({ ...query, rawBody })}
        />
      ) : (
        <>
          <InlineField label="Collection" labelWidth={16}>
            {listing?.canList ? (
              <Combobox
                id="query-editor-collection"
                createCustomValue
                options={listing.collections.map((name) => ({ value: name }))}
                value={query.collection}
                onChange={(option) => changeAndRun({ ...query, collection: option.value })}
              />
            ) : (
              <Input
                id="query-editor-collection"
                value={query.collection ?? ''}
                onChange={(e) => onChange({ ...query, collection: e.currentTarget.value })}
                onBlur={onRunQuery}
              />
            )}
          </InlineField>
          {builderFields.map(([key, label]) => textField(key, label))}
          {numberField('perPage', 'Per page', 250)}
          {textField('timeField', 'Time field')}
          <InlineField label="Format" labelWidth={16}>
            <RadioButtonGroup<Format>
              options={formats}
              value={query.format}
              onChange={(format) =>
                changeAndRun(
                  format === 'timeseries'
                    ? { ...query, format, bucketMode: query.bucketMode ?? 'hits' }
                    : { ...query, format }
                )
              }
            />
          </InlineField>
          {query.format === 'timeseries' && (
            <>
              <InlineField label="Bucket mode" labelWidth={16}>
                <RadioButtonGroup<BucketMode>
                  options={bucketModes}
                  value={query.bucketMode}
                  onChange={(bucketMode) => changeAndRun({ ...query, bucketMode })}
                />
              </InlineField>
              {textField('minInterval', 'Min interval')}
              {query.bucketMode !== 'groupBy' && textField('valueField', 'Value field')}
              {query.bucketMode !== 'groupBy' && (
                <InlineField label="Stats" labelWidth={16}>
                  <InlineSwitch
                    value={query.stats}
                    onChange={(e) => changeAndRun({ ...query, stats: e.currentTarget.checked })}
                  />
                </InlineField>
              )}
              {query.bucketMode !== 'rangeFacet' && numberField('maxPages', 'Max pages')}
            </>
          )}
        </>
      )}
    </Stack>
  );
}
