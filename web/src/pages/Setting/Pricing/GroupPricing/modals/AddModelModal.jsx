import React, { useState } from 'react';
import { Modal, Form, Divider, Typography } from '@douyinfe/semi-ui';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

// 解析数字输入，允许留空返回 null
const parseNumberInput = (value) => {
  if (value === '' || value === null || value === undefined) {
    return null;
  }
  const num = parseFloat(value);
  return isNaN(num) ? null : num;
};

const AddModelModal = ({ visible, onOk, onCancel, loading }) => {
  const { t } = useTranslation();
  const [formApi, setFormApi] = useState(null);
  const [fixedPriceValue, setFixedPriceValue] = useState('');
  // 追踪用户是否手动修改过缓存价格字段
  const [cacheModified, setCacheModified] = useState({
    cache_price: false,
    cache_creation_price: false,
  });

  const isDisabled = fixedPriceValue !== '' && parseFloat(fixedPriceValue) > 0;

  // 当输入价格变化时自动计算缓存价格
  const handleInputPriceChange = (value) => {
    if (!formApi) return;
    const input = parseFloat(value);
    if (!isNaN(input) && input > 0) {
      // 仅当用户未手动修改过时才自动填充
      if (!cacheModified.cache_price) {
        formApi.setValue('cache_price', (input * 0.1).toFixed(4));
      }
      if (!cacheModified.cache_creation_price) {
        formApi.setValue('cache_creation_price', (input * 1.25).toFixed(4));
      }
    }
  };

  const handleOk = () => {
    if (!formApi) return;
    const values = formApi.getValues();
    if (!values.model) return;

    const processedValues = {
      model: values.model,
      input_price: parseNumberInput(values.input_price),
      output_price: parseNumberInput(values.output_price),
      cache_price: parseNumberInput(values.cache_price),
      cache_creation_price: parseNumberInput(values.cache_creation_price),
      image_price: parseNumberInput(values.image_price),
      audio_input_price: parseNumberInput(values.audio_input_price),
      audio_output_price: parseNumberInput(values.audio_output_price),
      fixed_price: parseNumberInput(values.fixed_price),
      quota_type: values.quota_type || 0,
    };

    if (processedValues.fixed_price && processedValues.fixed_price > 0) {
      processedValues.quota_type = 1;
    }

    onOk(processedValues);
  };

  const handleCancel = () => {
    setCacheModified({ cache_price: false, cache_creation_price: false });
    setFixedPriceValue('');
    onCancel();
  };

  return (
    <Modal
      title={t('添加模型到分组')}
      visible={visible}
      onOk={handleOk}
      onCancel={handleCancel}
      okText={t('添加')}
      cancelText={t('取消')}
      confirmLoading={loading}
    >
      <Form
        getFormApi={(api) => setFormApi(api)}
        labelPosition="left"
        labelWidth={120}
      >
        <Form.Input
          field="model"
          label={t('模型名称')}
          placeholder={t('例如: gpt-4o')}
          rules={[{ required: true }]}
        />
        <Form.Input
          field="fixed_price"
          label={t('固定价格')}
          placeholder={t('留空使用按量计费')}
          onChange={(val) => setFixedPriceValue(val)}
          extraText={t('设置固定价格后将使用按次计费，其他价格字段无效')}
        />
        <Form.Input
          field="input_price"
          label={t('输入价格')}
          placeholder={t('留空表示未配置')}
          disabled={isDisabled}
          onChange={handleInputPriceChange}
        />
        <Form.Input
          field="output_price"
          label={t('输出价格')}
          placeholder={t('留空表示未配置')}
          disabled={isDisabled}
        />
        <Form.Input
          field="cache_price"
          label={t('缓存价格')}
          placeholder={t('留空表示未配置')}
          disabled={isDisabled}
          extraText={t('未设置则以输入价格×0.1为默认值')}
          onChange={() => setCacheModified(prev => ({ ...prev, cache_price: true }))}
        />
        <Form.Input
          field="cache_creation_price"
          label={t('缓存创建价格')}
          placeholder={t('留空表示未配置')}
          disabled={isDisabled}
          extraText={t('未设置则以输入价格×1.25为默认值')}
          onChange={() => setCacheModified(prev => ({ ...prev, cache_creation_price: true }))}
        />
        <Divider style={{ margin: '16px 0 8px' }}>
          <Text type="tertiary" size="small">{t('特殊价格（可选）')}</Text>
        </Divider>
        <Form.Input
          field="image_price"
          label={t('图片价格')}
          placeholder={t('留空表示未配置')}
          disabled={isDisabled}
        />
        <Form.Input
          field="audio_input_price"
          label={t('音频输入价格')}
          placeholder={t('留空表示未配置')}
          disabled={isDisabled}
        />
        <Form.Input
          field="audio_output_price"
          label={t('音频输出价格')}
          placeholder={t('留空表示未配置')}
          disabled={isDisabled}
        />
      </Form>
    </Modal>
  );
};

AddModelModal.propTypes = {
  visible: PropTypes.bool.isRequired,
  onOk: PropTypes.func.isRequired,
  onCancel: PropTypes.func.isRequired,
  loading: PropTypes.bool,
};

export default AddModelModal;
