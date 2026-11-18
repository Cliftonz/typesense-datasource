import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { DataSourceSettings } from '@grafana/data';
import { ConfigEditor } from './ConfigEditor';
import { MyDataSourceOptions, MySecureJsonData } from '../types';

const settings = (
  jsonData: Partial<MyDataSourceOptions> = {}
): DataSourceSettings<MyDataSourceOptions, MySecureJsonData> =>
  ({
    jsonData,
    secureJsonFields: {},
    secureJsonData: {},
  }) as unknown as DataSourceSettings<MyDataSourceOptions, MySecureJsonData>;

test('writes the URL to jsonData.url', () => {
  const onOptionsChange = jest.fn();
  render(<ConfigEditor options={settings()} onOptionsChange={onOptionsChange} />);

  fireEvent.change(screen.getByRole('textbox', { name: 'URL' }), { target: { value: 'http://localhost:8108' } });

  expect(onOptionsChange.mock.calls[0][0].jsonData.url).toBe('http://localhost:8108');
});

test('writes the API key to secureJsonData.apiKey only', () => {
  const onOptionsChange = jest.fn();
  render(<ConfigEditor options={settings()} onOptionsChange={onOptionsChange} />);

  fireEvent.change(screen.getByPlaceholderText('Enter your API key'), { target: { value: 'test-key' } });

  const changed = onOptionsChange.mock.calls[0][0];
  expect(changed.secureJsonData.apiKey).toBe('test-key');
  expect(JSON.stringify(changed.jsonData)).not.toContain('test-key');
});

test('resetting a configured API key clears it', () => {
  const onOptionsChange = jest.fn();
  const configured = { ...settings(), secureJsonFields: { apiKey: true } };
  render(<ConfigEditor options={configured} onOptionsChange={onOptionsChange} />);

  fireEvent.click(screen.getByRole('button', { name: 'Reset' }));

  const changed = onOptionsChange.mock.calls[0][0];
  expect(changed.secureJsonFields.apiKey).toBe(false);
  expect(changed.secureJsonData.apiKey).toBe('');
});

test('time unit shows Seconds when unset and writes "ms" when Milliseconds is chosen', () => {
  const onOptionsChange = jest.fn();
  render(<ConfigEditor options={settings()} onOptionsChange={onOptionsChange} />);

  const timeUnit = screen.getByRole('combobox', { name: 'Time unit' });

  expect(timeUnit).toHaveValue('Seconds');
  fireEvent.change(timeUnit, { target: { value: 'Milliseconds' } });
  fireEvent.keyDown(timeUnit, { key: 'Enter' });

  expect(onOptionsChange.mock.calls[0][0].jsonData.timeUnit).toBe('ms');
});

test('writes the default collection to jsonData.defaultCollection', () => {
  const onOptionsChange = jest.fn();
  render(<ConfigEditor options={settings()} onOptionsChange={onOptionsChange} />);

  fireEvent.change(screen.getByRole('textbox', { name: 'Default collection' }), { target: { value: 'events' } });

  expect(onOptionsChange.mock.calls[0][0].jsonData.defaultCollection).toBe('events');
});

test('max pages is empty with placeholder 40 when unset and writes a number to jsonData.maxPages', () => {
  const onOptionsChange = jest.fn();
  render(<ConfigEditor options={settings()} onOptionsChange={onOptionsChange} />);
  const maxPages = screen.getByRole('spinbutton', { name: 'Max pages' });

  expect(maxPages).toHaveValue(null);
  expect(maxPages).toHaveAttribute('placeholder', '40');
  fireEvent.change(maxPages, { target: { value: '10' } });

  expect(onOptionsChange.mock.calls[0][0].jsonData.maxPages).toBe(10);
});

test('clearing max pages removes maxPages from jsonData', () => {
  const onOptionsChange = jest.fn();
  render(<ConfigEditor options={settings({ maxPages: 10 })} onOptionsChange={onOptionsChange} />);

  fireEvent.change(screen.getByRole('spinbutton', { name: 'Max pages' }), { target: { value: '' } });

  expect(onOptionsChange.mock.calls[0][0].jsonData).not.toHaveProperty('maxPages');
});

test.each(['0', '-3', '2.5'])('max pages does not write %s', (value) => {
  const onOptionsChange = jest.fn();
  render(<ConfigEditor options={settings({ maxPages: 10 })} onOptionsChange={onOptionsChange} />);
  const maxPages = screen.getByRole('spinbutton', { name: 'Max pages' });

  expect(maxPages).toHaveAttribute('min', '1');
  expect(maxPages).toHaveAttribute('step', '1');
  fireEvent.change(maxPages, { target: { value } });

  expect(onOptionsChange).not.toHaveBeenCalled();
});

test('clearing default collection removes defaultCollection from jsonData', () => {
  const onOptionsChange = jest.fn();
  render(<ConfigEditor options={settings({ defaultCollection: 'events' })} onOptionsChange={onOptionsChange} />);

  fireEvent.change(screen.getByRole('textbox', { name: 'Default collection' }), { target: { value: '' } });

  expect(onOptionsChange.mock.calls[0][0].jsonData).not.toHaveProperty('defaultCollection');
});

test('the API key is not a required field', () => {
  render(<ConfigEditor options={settings()} onOptionsChange={jest.fn()} />);

  expect(screen.getByPlaceholderText('Enter your API key')).not.toBeRequired();
});
