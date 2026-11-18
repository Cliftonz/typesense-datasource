import { test, expect } from '@grafana/plugin-e2e';
import { MyDataSourceOptions, MySecureJsonData } from '../src/types';

test('saving persists the URL and API key', async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource<MyDataSourceOptions, MySecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByRole('textbox', { name: 'URL' }).fill(ds.jsonData.url!);
  await page.getByRole('textbox', { name: 'API Key' }).fill(ds.secureJsonData!.apiKey!);
  await configPage.saveAndTest();

  await configPage.goto();

  await expect(page.getByRole('textbox', { name: 'URL' })).toHaveValue(ds.jsonData.url!);
  await expect(page.getByRole('textbox', { name: 'API Key' })).toHaveValue('configured');
});

for (const [label, description] of [
  ['Time unit', 'Unit of the timestamp fields in your collections'],
  ['Default collection', "Used by Save & test when the API key can't list collections"],
]) {
  test(`${label} label explains the field`, async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
    const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
    await createDataSourceConfigPage({ type: ds.type });

    await page.locator('label', { hasText: label }).locator('svg').hover();

    await expect(page.getByRole('tooltip')).toHaveText(description);
  });
}
