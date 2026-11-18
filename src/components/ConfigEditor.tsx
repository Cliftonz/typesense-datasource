import React, { ChangeEvent } from 'react';
import { Combobox, ComboboxOption, InlineField, Input, SecretInput } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { MyDataSourceOptions, MySecureJsonData, TimeUnit } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions, MySecureJsonData> {}

const timeUnitOptions: Array<ComboboxOption<TimeUnit>> = [
  { label: 'Seconds', value: 's' },
  { label: 'Milliseconds', value: 'ms' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        url: event.target.value,
      },
    });
  };

  const onTimeUnitChange = (option: ComboboxOption<TimeUnit>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        timeUnit: option.value,
      },
    });
  };

  const onDefaultCollectionChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { defaultCollection, ...withoutDefaultCollection } = jsonData;
    onOptionsChange({
      ...options,
      jsonData:
        event.target.value === ''
          ? withoutDefaultCollection
          : { ...withoutDefaultCollection, defaultCollection: event.target.value },
    });
  };

  const onMaxPagesChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { maxPages, ...withoutMaxPages } = jsonData;
    const value = Number(event.target.value);
    if (event.target.value !== '' && !(Number.isInteger(value) && value >= 1)) {
      return;
    }
    onOptionsChange({
      ...options,
      jsonData: event.target.value === '' ? withoutMaxPages : { ...withoutMaxPages, maxPages: value },
    });
  };

  const onAPIKeyChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        apiKey: event.target.value,
      },
    });
  };

  const onResetAPIKey = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        apiKey: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        apiKey: '',
      },
    });
  };

  return (
    <>
      <InlineField label="URL" labelWidth={20}>
        <Input id="config-editor-url" onChange={onURLChange} value={jsonData.url} width={40} />
      </InlineField>
      <InlineField label="API Key" labelWidth={20}>
        <SecretInput
          id="config-editor-api-key"
          isConfigured={secureJsonFields.apiKey}
          value={secureJsonData?.apiKey}
          placeholder="Enter your API key"
          width={40}
          onReset={onResetAPIKey}
          onChange={onAPIKeyChange}
        />
      </InlineField>
      <InlineField label="Time unit" labelWidth={20} tooltip="Unit of the timestamp fields in your collections">
        <Combobox
          id="config-editor-time-unit"
          options={timeUnitOptions}
          value={jsonData.timeUnit ?? 's'}
          onChange={onTimeUnitChange}
          width={40}
        />
      </InlineField>
      <InlineField
        label="Default collection"
        labelWidth={20}
        tooltip="Used by Save & test when the API key can't list collections"
      >
        <Input
          id="config-editor-default-collection"
          onChange={onDefaultCollectionChange}
          value={jsonData.defaultCollection}
          width={40}
        />
      </InlineField>
      <InlineField label="Max pages" labelWidth={20}>
        <Input
          id="config-editor-max-pages"
          type="number"
          min={1}
          step={1}
          onChange={onMaxPagesChange}
          value={jsonData.maxPages ?? ''}
          placeholder="40"
          width={40}
        />
      </InlineField>
    </>
  );
}
