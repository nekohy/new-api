import React, { useState } from 'react';
import { Typography, Input, Button, Space } from '@douyinfe/semi-ui';
import { IconEdit, IconTick, IconClose } from '@douyinfe/semi-icons';
import PropTypes from 'prop-types';

/**
 * 可编辑描述组件
 *
 * 用于在表格或列表中快速编辑文本描述
 */
function EditableDescription({
  value,
  onSave,
  placeholder = '暂无描述',
  maxLength = 500,
  loading = false,
  maxWidth = 300
}) {
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editValue, setEditValue] = useState(value || '');

  const handleSave = async () => {
    const trimmedValue = editValue.trim();

    // 值未改变，直接退出编辑
    if (trimmedValue === (value || '')) {
      setEditing(false);
      return;
    }

    // 长度验证
    if (trimmedValue.length > maxLength) {
      setEditValue(value || '');
      setEditing(false);
      return;
    }

    setSaving(true);
    try {
      await onSave(trimmedValue);
      setEditing(false);
    } catch (error) {
      // 恢复原值，错误提示由调用方负责
      setEditValue(value || '');
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    setEditValue(value || '');
    setEditing(false);
  };

  const handleEditStart = () => {
    setEditValue(value || '');
    setEditing(true);
  };

  if (editing) {
    return (
      <Space>
        <Input
          value={editValue}
          onChange={setEditValue}
          size="small"
          style={{ width: maxWidth - 60 }}
          disabled={saving || loading}
          onKeyDown={(e) => {
            if (e.key === 'Enter') handleSave();
            if (e.key === 'Escape') handleCancel();
          }}
          autoFocus
        />
        <Button
          size="small"
          icon={<IconTick />}
          onClick={handleSave}
          loading={saving || loading}
          type="primary"
          theme="solid"
        />
        <Button
          size="small"
          icon={<IconClose />}
          onClick={handleCancel}
          disabled={saving || loading}
        />
      </Space>
    );
  }

  return (
    <div
      onClick={handleEditStart}
      style={{
        maxWidth,
        cursor: 'pointer',
        display: 'flex',
        alignItems: 'center',
        gap: 4,
      }}
    >
      <Typography.Text
        ellipsis={{
          showTooltip: {
            opts: { content: value || placeholder }
          }
        }}
        style={{
          color: !value ? 'var(--semi-color-text-2)' : undefined,
          flex: 1,
          pointerEvents: 'none' // 禁用文本自身交互，让事件穿透到父级 div
        }}
      >
        {value || placeholder}
      </Typography.Text>
      <IconEdit size="small" style={{ color: 'var(--semi-color-text-2)', flexShrink: 0 }} />
    </div>
  );
}

EditableDescription.propTypes = {
  value: PropTypes.string,
  onSave: PropTypes.func.isRequired,
  placeholder: PropTypes.string,
  maxLength: PropTypes.number,
  loading: PropTypes.bool,
  maxWidth: PropTypes.number,
};

export default EditableDescription;
