import React, { useMemo } from 'react';
import {
  Table,
  Select,
  Typography,
  Tag,
  Space,
  Button,
  Tooltip,
} from '@douyinfe/semi-ui';
import { IconUndo, IconHelpCircle } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const ModelGroupConfigurator = ({
  models = [],
  value = { defaultGroups: [], specificRules: {} },
  onChange,
  allGroups = [],
  disabled = false,
}) => {
  const { t } = useTranslation();

  // 缓存 optionList 避免重复渲染
  const groupOptions = useMemo(() =>
    allGroups.map(g => ({ label: g, value: g })),
    [allGroups]
  );

  const handleDefaultChange = (v) => {
    onChange({ ...value, defaultGroups: v });
  };

  const handleSpecificChange = (model, v) => {
    onChange({
      ...value,
      specificRules: { ...value.specificRules, [model]: v },
    });
  };

  const handleReset = (model) => {
    const newRules = { ...value.specificRules };
    delete newRules[model];
    onChange({ ...value, specificRules: newRules });
  };

  const columns = [
    {
      title: t('模型名称'),
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: (text) => <Text strong>{text}</Text>,
    },
    {
      title: t('分组配置'),
      dataIndex: 'groups',
      key: 'groups',
      render: (text, record) => {
        const isOverridden = Object.prototype.hasOwnProperty.call(
          value.specificRules,
          record.name,
        );
        const currentGroups = isOverridden
          ? value.specificRules[record.name]
          : value.defaultGroups;

        return (
          <Select
            multiple
            value={currentGroups}
            onChange={(v) => handleSpecificChange(record.name, v)}
            optionList={groupOptions}
            placeholder={t('请选择分组')}
            disabled={disabled}
            filter
            maxTagCount={3}
            style={{ width: '100%' }}
            emptyContent={t('无可用分组')}
          />
        );
      },
    },
    {
      title: t('状态'),
      key: 'status',
      width: 120,
      render: (text, record) => {
        const isOverridden = Object.prototype.hasOwnProperty.call(
          value.specificRules,
          record.name,
        );
        return (
          <Space>
            {isOverridden ? (
              <Tag color='orange'>{t('自定义')}</Tag>
            ) : (
              <Tag color='grey'>{t('继承默认')}</Tag>
            )}
            {isOverridden && (
              <Tooltip content={t('重置为默认分组')}>
                <Button
                  icon={<IconUndo />}
                  type='tertiary'
                  theme='borderless'
                  size='small'
                  onClick={() => handleReset(record.name)}
                  disabled={disabled}
                />
              </Tooltip>
            )}
          </Space>
        );
      },
    },
  ];

  const dataSource = models.map((m) => ({ key: m, name: m }));

  return (
    <div className='model-group-config'>
      <div
        style={{
          backgroundColor: 'var(--semi-color-fill-0)',
          padding: 16,
          borderRadius: 'var(--semi-border-radius-medium)',
          marginBottom: 16,
        }}
      >
        <div style={{ marginBottom: 8 }}>
          <Text strong>{t('默认分组')}</Text>
          <Tooltip content={t('所有未单独配置的模型将使用此分组')}>
            <IconHelpCircle
              style={{ marginLeft: 4, color: 'var(--semi-color-text-2)' }}
            />
          </Tooltip>
        </div>
        <Select
          multiple
          value={value.defaultGroups}
          onChange={handleDefaultChange}
          optionList={groupOptions}
          placeholder={t('请选择分组')}
          disabled={disabled}
          filter
          maxTagCount={5}
          style={{ width: '100%' }}
          emptyContent={t('无可用分组')}
        />
        <div style={{ marginTop: 8 }}>
          <Text type='tertiary' size='small'>
            {t('此设置将应用于列表中的所有模型，除非下方单独覆盖')}
          </Text>
        </div>
      </div>

      {models.length > 0 && (
        <Table
          columns={columns}
          dataSource={dataSource}
          pagination={{ pageSize: 10 }}
          size='small'
          empty={t('请先选择模型')}
        />
      )}
    </div>
  );
};

export default ModelGroupConfigurator;
