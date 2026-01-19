/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState, useMemo, useRef } from 'react';
import {
  Table,
  Button,
  Space,
  Input,
  Popconfirm,
  Typography,
  Modal,
  Tag,
  Empty,
  Divider,
  Banner,
  Switch,
  Select,
  Tooltip,
} from '@douyinfe/semi-ui';
import { IconSearch, IconPlus, IconDelete, IconSync, IconAlertTriangle, IconSave, IconRefresh, IconInbox } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import useGroupPrices from '../hooks/useGroupPrices';
import SelectableButtonGroup from '../../../../components/common/ui/SelectableButtonGroup';
import FloatingActionToolbar from '../../../../components/common/FloatingActionToolbar';
import {
  CreateGroupModal,
  AddModelModal,
  SyncFromGroupModal,
  BatchMultiplierModal,
} from './modals';

const { Text, Title } = Typography;

// 解析数字输入，允许留空返回 null
const parseNumberInput = (value) => {
  if (value === '' || value === null || value === undefined) {
    return null;
  }
  const num = parseFloat(value);
  return isNaN(num) ? null : num;
};

// 格式化数字显示，区分 null（未设置）和 0（免费）
const formatNumber = (val) => {
  if (val === null || val === undefined) return null; // null 表示未设置
  return val.toFixed(4);
};

const GroupPricing = () => {
  const { t } = useTranslation();
  const [selectedGroup, setSelectedGroup] = useState('');
  const {
    groups,
    prices,
    loading,
    fetchGroups,
    createGroup,
    deleteGroup,
    addModelToGroup,
    updateGroupPrice,
    removeModelFromGroup,
    batchRemoveModels,
    syncFromGroup,
    applyMultiplier,
    // 批量保存相关
    tempChanges,
    updateLocalPrice,
    discardChanges,
    batchSavePrices,
    hasUnsavedChanges,
    unsavedCount,
  } = useGroupPrices(selectedGroup);

  const [searchText, setSearchText] = useState('');
  const [createGroupModalVisible, setCreateGroupModalVisible] = useState(false);
  const [addModelModalVisible, setAddModelModalVisible] = useState(false);
  const [syncModalVisible, setSyncModalVisible] = useState(false);
  const [multiplierModalVisible, setMultiplierModalVisible] = useState(false);
  const [showSpecialPriceColumns, setShowSpecialPriceColumns] = useState(false);
  const [configFilter, setConfigFilter] = useState('all'); // 'all', 'configured', 'unconfigured'
  const [pendingGroupSwitch, setPendingGroupSwitch] = useState(null); // 待切换的分组名
  const [saveLoading, setSaveLoading] = useState(false);

  // 多选状态
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);

  // 表内编辑状态
  const [editingCell, setEditingCell] = useState({ model: '', field: '' });
  const [editingValue, setEditingValue] = useState('');
  const inputRef = useRef(null);

  useEffect(() => {
    fetchGroups();
  }, []);

  // 切换分组时清空选择
  useEffect(() => {
    setSelectedRowKeys([]);
    setConfigFilter('all');
  }, [selectedGroup]);

  // 检查分组价格是否已配置（至少有一个价格字段非 null）
  const isGroupPriceConfigured = (item) => {
    return item.input_price !== null ||
      item.output_price !== null ||
      item.cache_price !== null ||
      item.cache_creation_price !== null ||
      item.image_price !== null ||
      item.audio_input_price !== null ||
      item.audio_output_price !== null ||
      item.fixed_price !== null ||
      item.quota_type !== null;
  };

  // 统计已配置和未配置数量
  const { configuredCount, unconfiguredCount } = useMemo(() => {
    let configured = 0;
    let unconfigured = 0;
    prices.forEach((item) => {
      if (isGroupPriceConfigured(item)) {
        configured++;
      } else {
        unconfigured++;
      }
    });
    return { configuredCount: configured, unconfiguredCount: unconfigured };
  }, [prices]);

  // 合并原始数据与临时变更，用于显示
  const mergedPrices = useMemo(() => {
    return prices.map((item) => {
      const changes = tempChanges[item.model];
      if (!changes) return { ...item, _isModified: false };
      return {
        ...item,
        ...changes,
        _isModified: true,
      };
    });
  }, [prices, tempChanges]);

  const filteredPrices = useMemo(() => {
    return mergedPrices.filter((item) => {
      // 搜索过滤
      if (searchText && !item.model.toLowerCase().includes(searchText.toLowerCase())) {
        return false;
      }
      // 配置状态过滤
      const isConfigured = isGroupPriceConfigured(item);
      if (configFilter === 'configured' && !isConfigured) {
        return false;
      }
      if (configFilter === 'unconfigured' && isConfigured) {
        return false;
      }
      return true;
    });
  }, [mergedPrices, searchText, configFilter]);

  // 开始编辑单元格
  const startCellEdit = (model, field, currentValue) => {
    if (editingCell.model && editingCell.field) {
      saveCellEdit();
    }
    setEditingCell({ model, field });
    setEditingValue(currentValue === null || currentValue === undefined ? '' : String(currentValue));
    setTimeout(() => inputRef.current?.focus(), 0);
  };

  // 保存单元格编辑（改为本地更新，不触发 API）
  const saveCellEdit = () => {
    if (!editingCell.model || !editingCell.field) return;

    const { model, field } = editingCell;
    const newValue = parseNumberInput(editingValue);

    // 获取当前显示值（原始数据 + 临时变更）
    const currentRecord = prices.find(p => p.model === model);
    const currentChanges = tempChanges[model] || {};
    const currentValue = currentChanges[field] !== undefined ? currentChanges[field] : (currentRecord ? currentRecord[field] : null);

    // 值未变化则不更新
    if (newValue === currentValue || (newValue === null && (currentValue === null || currentValue === undefined))) {
      setEditingCell({ model: '', field: '' });
      setEditingValue('');
      return;
    }

    // 更新本地状态
    updateLocalPrice(model, field, newValue);

    // 如果设置了固定价格，自动设置 quota_type 为按次计费
    if (field === 'fixed_price' && newValue !== null && newValue > 0) {
      updateLocalPrice(model, 'quota_type', 1);
    }

    setEditingCell({ model: '', field: '' });
    setEditingValue('');
  };

  // 处理键盘事件
  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      saveCellEdit();
    } else if (e.key === 'Escape') {
      setEditingCell({ model: '', field: '' });
      setEditingValue('');
    }
  };

  // 批量保存所有变更
  const handleBatchSave = async () => {
    if (!hasUnsavedChanges) return;
    setSaveLoading(true);
    await batchSavePrices(selectedGroup);
    setSaveLoading(false);
  };

  // 丢弃所有变更
  const handleDiscardChanges = () => {
    discardChanges();
  };

  // 切换分组（带未保存确认）
  const handleGroupChange = (newGroup) => {
    if (hasUnsavedChanges && newGroup !== selectedGroup) {
      setPendingGroupSwitch(newGroup);
    } else {
      setSelectedGroup(newGroup);
    }
  };

  // 确认切换分组（丢弃未保存变更）
  const confirmGroupSwitch = () => {
    discardChanges();
    setSelectedGroup(pendingGroupSwitch);
    setPendingGroupSwitch(null);
  };

  // 取消切换分组
  const cancelGroupSwitch = () => {
    setPendingGroupSwitch(null);
  };

  // 判断字段是否应该禁用
  const isFieldDisabled = (record, field) => {
    if (!record.fixed_price || record.fixed_price <= 0) {
      return false;
    }
    return ['input_price', 'output_price', 'cache_price', 'cache_creation_price', 'image_price', 'audio_input_price', 'audio_output_price'].includes(field);
  };

  const handleCreateGroup = async (groupName) => {
    const success = await createGroup(groupName);
    if (success) {
      setCreateGroupModalVisible(false);
    }
  };

  const handleDeleteGroup = async (groupName) => {
    const success = await deleteGroup(groupName);
    if (success) {
      // 刷新分组列表
      await fetchGroups();

      if (selectedGroup === groupName) {
        // 智能选择下一个分组
        if (groups.length > 0) {
          // 优先选择 default，否则选第一个
          const nextGroup = groups.includes('default') ? 'default' : groups[0];
          setSelectedGroup(nextGroup);
        } else {
          // 进入空状态
          setSelectedGroup('');
        }
      }
    }
  };

  const handleAddModel = async (processedValues) => {
    const success = await addModelToGroup(selectedGroup, processedValues);
    if (success) {
      setAddModelModalVisible(false);
    }
  };

  // 批量删除
  const handleBatchRemove = async () => {
    if (selectedRowKeys.length === 0) return;

    if (batchRemoveModels) {
      await batchRemoveModels(selectedGroup, selectedRowKeys);
    } else {
      for (const model of selectedRowKeys) {
        await removeModelFromGroup(selectedGroup, model);
      }
    }
    setSelectedRowKeys([]);
  };

  // 从源分组同步价格
  const handleSyncFromGroup = async ({ sourceGroup, multiplier, mode }) => {
    const success = await syncFromGroup(selectedGroup, sourceGroup, multiplier, mode);
    if (success) {
      setSyncModalVisible(false);
    }
  };

  // 从指定分组同步价格
  const handleApplyMultiplier = async ({ sourceGroup, multiplier }) => {
    if (selectedRowKeys.length === 0) return;
    const success = await applyMultiplier(selectedGroup, selectedRowKeys, multiplier, sourceGroup);
    if (success) {
      setMultiplierModalVisible(false);
      setSelectedRowKeys([]);
    }
  };

  // 渲染可编辑单元格，区分 null（未设置）和 0（免费）
  const renderEditableCell = (field, record, displayValue) => {
    const isEditing = editingCell.model === record.model && editingCell.field === field;
    const disabled = isFieldDisabled(record, field);
    const rawValue = record[field];
    const isUnset = rawValue === null || rawValue === undefined;
    const isFree = rawValue === 0;
    // 检查该字段是否被修改
    const isFieldModified = tempChanges[record.model] && tempChanges[record.model][field] !== undefined;

    if (isEditing) {
      return (
        <Input
          ref={inputRef}
          size="small"
          value={editingValue}
          onChange={setEditingValue}
          onBlur={saveCellEdit}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          style={{ width: '80px' }}
        />
      );
    }

    // 未设置：显示灰色 "-"
    // 免费（0）：显示绿色 "0.0000"
    // 有价格：显示正常数字
    let displayContent;
    let textStyle = {};
    let tooltip = disabled ? t('固定价格模式下不可编辑') : t('点击编辑');

    if (isUnset) {
      displayContent = '-';
      textStyle = { color: 'var(--semi-color-text-2)' };
      tooltip = disabled ? t('固定价格模式下不可编辑') : t('未设置，点击编辑');
    } else if (isFree) {
      displayContent = '0.0000';
      textStyle = { color: 'var(--semi-color-success)' };
      tooltip = disabled ? t('固定价格模式下不可编辑') : t('免费，点击编辑');
    } else {
      displayContent = displayValue;
    }

    // 已修改字段添加高亮背景
    const modifiedStyle = isFieldModified ? {
      backgroundColor: 'var(--semi-color-info-light-default)',
      borderRadius: '4px',
    } : {};

    return (
      <div
        onClick={() => !disabled && startCellEdit(record.model, field, rawValue)}
        style={{
          cursor: disabled ? 'not-allowed' : 'pointer',
          opacity: disabled ? 0.5 : 1,
          minHeight: '22px',
          padding: '2px 4px',
          borderRadius: '2px',
          ...textStyle,
          ...modifiedStyle,
        }}
        title={isFieldModified ? t('已修改，待保存') : tooltip}
      >
        {displayContent}
      </div>
    );
  };

  const baseColumns = [
    {
      title: t('模型'),
      dataIndex: 'model',
      width: 280,
      fixed: 'left',
      render: (text, record) => (
        <Space>
          <Text copyable>{text}</Text>
          {!isGroupPriceConfigured(record) && (
            <Tooltip content={t('价格未配置，点击编辑价格后自动标记为已配置')}>
              <Tag color="orange" size="small">
                <IconAlertTriangle size="small" style={{ marginRight: 2 }} />
                {t('未配置')}
              </Tag>
            </Tooltip>
          )}
        </Space>
      ),
      sorter: (a, b) => a.model.localeCompare(b.model),
    },
    {
      title: t('输入价格'),
      dataIndex: 'input_price',
      width: 100,
      render: (val, record) => renderEditableCell('input_price', record, formatNumber(val)),
    },
    {
      title: t('输出价格'),
      dataIndex: 'output_price',
      width: 100,
      render: (val, record) => renderEditableCell('output_price', record, formatNumber(val)),
    },
    {
      title: t('缓存价格'),
      dataIndex: 'cache_price',
      width: 100,
      render: (val, record) => renderEditableCell('cache_price', record, formatNumber(val)),
    },
    {
      title: t('缓存创建价格'),
      dataIndex: 'cache_creation_price',
      width: 120,
      render: (val, record) => renderEditableCell('cache_creation_price', record, formatNumber(val)),
    },
  ];

  const specialPriceColumns = [
    {
      title: t('图片价格'),
      dataIndex: 'image_price',
      width: 100,
      render: (val, record) => renderEditableCell('image_price', record, formatNumber(val)),
    },
    {
      title: t('音频输入价格'),
      dataIndex: 'audio_input_price',
      width: 120,
      render: (val, record) => renderEditableCell('audio_input_price', record, formatNumber(val)),
    },
    {
      title: t('音频输出价格'),
      dataIndex: 'audio_output_price',
      width: 120,
      render: (val, record) => renderEditableCell('audio_output_price', record, formatNumber(val)),
    },
  ];

  const fixedColumns = [
    {
      title: t('固定价格'),
      dataIndex: 'fixed_price',
      width: 100,
      render: (val, record) => {
        const isEditing = editingCell.model === record.model && editingCell.field === 'fixed_price';
        const isUnset = val === null || val === undefined;
        const isFree = val === 0;

        if (isEditing) {
          return (
            <Input
              ref={inputRef}
              size="small"
              value={editingValue}
              onChange={setEditingValue}
              onBlur={saveCellEdit}
              onKeyDown={handleKeyDown}
              style={{ width: '80px' }}
            />
          );
        }

        let displayContent;
        let textStyle = {};
        let tooltip = t('点击编辑');

        if (isUnset) {
          displayContent = '-';
          textStyle = { color: 'var(--semi-color-text-2)' };
          tooltip = t('未设置（按量计费），点击编辑');
        } else if (isFree) {
          displayContent = '$0';
          textStyle = { color: 'var(--semi-color-success)' };
          tooltip = t('免费，点击编辑');
        } else {
          displayContent = `$${val.toFixed(4)}`;
        }

        return (
          <div
            onClick={() => startCellEdit(record.model, 'fixed_price', val)}
            style={{
              cursor: 'pointer',
              minHeight: '22px',
              padding: '2px 4px',
              borderRadius: '2px',
              ...textStyle,
            }}
            title={tooltip}
          >
            {displayContent}
          </div>
        );
      },
    },
    {
      title: t('计费类型'),
      dataIndex: 'quota_type',
      width: 80,
      render: (val) => (
        <Tag color={val === 0 ? 'blue' : 'green'} size="small">
          {val === 0 ? t('按量') : t('按次')}
        </Tag>
      ),
    },
  ];

  const columns = useMemo(() => {
    if (showSpecialPriceColumns) {
      return [...baseColumns, ...specialPriceColumns, ...fixedColumns];
    }
    return [...baseColumns, ...fixedColumns];
  }, [showSpecialPriceColumns, editingCell, editingValue]);

  // 多选配置
  const rowSelection = {
    selectedRowKeys,
    onChange: (keys) => setSelectedRowKeys(keys),
  };

  // 构建分组选择器的数据项
  const groupItems = useMemo(() => {
    return groups.map((group) => ({
      value: group,
      label: group,
      tagCount: group === selectedGroup ? prices.length : undefined,
    }));
  }, [groups, selectedGroup, prices.length]);

  const renderGroupList = () => (
    <div style={{ marginBottom: 16 }}>
      <Space style={{ marginBottom: 8, width: '100%', justifyContent: 'space-between' }}>
        <div />
        <Button
          icon={<IconPlus />}
          size="small"
          onClick={() => setCreateGroupModalVisible(true)}
        >
          {t('新建分组')}
        </Button>
      </Space>
      {groups.length > 0 ? (
        <SelectableButtonGroup
          title={t('定价分组')}
          items={groupItems}
          activeValue={selectedGroup}
          onChange={handleGroupChange}
          loading={loading}
          t={t}
          collapsible={groups.length > 12}
        />
      ) : (
        <Empty description={t('暂无分组，请创建')} style={{ padding: '20px 0' }} />
      )}
    </div>
  );

  return (
    <div style={{ padding: '16px 0' }}>
      <Banner
        type="info"
        description={t('分组定价用于为不同分组配置不同的模型价格。')}
        style={{ marginBottom: 16 }}
      />

      {renderGroupList()}

      <Divider />

      {selectedGroup ? (
        <>
          <Space style={{ marginBottom: 16, justifyContent: 'space-between', width: '100%' }}>
            <Space>
              <Title heading={6}>
                {t('分组')}: <Tag color="blue">{selectedGroup}</Tag>
                <Text type="tertiary" style={{ marginLeft: 8 }}>
                  ({prices.length} {t('个模型')})
                </Text>
              </Title>
              <Popconfirm
                title={
                  selectedGroup === 'default'
                    ? t('default 为系统内置的默认分组，删除之后可能导致原用 default 的渠道需指定分组，需要确认吗？')
                    : t('确定删除分组 "{{group}}" 及其所有定价配置？', { group: selectedGroup })
                }
                onConfirm={() => handleDeleteGroup(selectedGroup)}
                okType="danger"
              >
                <Button icon={<IconDelete />} type="danger" size="small">
                  {t('删除分组')}
                </Button>
              </Popconfirm>
            </Space>
            <Button
              icon={<IconSync />}
              type="primary"
              onClick={() => setSyncModalVisible(true)}
            >
              {t('同步模型价格')}
            </Button>
          </Space>

          {/* 批量操作栏 */}
          {selectedRowKeys.length > 0 && (
            <Banner
              type="warning"
              style={{ marginBottom: 16 }}
              description={
                <Space>
                  <Text>{t('已选择 {{count}} 项', { count: selectedRowKeys.length })}</Text>
                  <Button
                    icon={<IconSync />}
                    size="small"
                    onClick={() => setMultiplierModalVisible(true)}
                  >
                    {t('同步指定分组')}
                  </Button>
                  <Popconfirm
                    title={t('确定从分组移除选中的 {{count}} 个模型？', { count: selectedRowKeys.length })}
                    onConfirm={handleBatchRemove}
                  >
                    <Button icon={<IconDelete />} type="danger" size="small">
                      {t('批量移除')}
                    </Button>
                  </Popconfirm>
                  <Button size="small" onClick={() => setSelectedRowKeys([])}>
                    {t('取消选择')}
                  </Button>
                </Space>
              }
            />
          )}

          {unconfiguredCount > 0 && configFilter !== 'configured' && (
            <Banner
              type="warning"
              icon={<IconAlertTriangle />}
              description={t('有 {{count}} 个模型价格未配置，请点击价格列进行设置', { count: unconfiguredCount })}
              style={{ marginBottom: 16 }}
            />
          )}
          <Space style={{ marginBottom: 16, flexWrap: 'wrap' }}>
            <Input
              prefix={<IconSearch />}
              placeholder={t('搜索模型')}
              value={searchText}
              onChange={setSearchText}
              style={{ width: 250 }}
            />
            <Select
              value={configFilter}
              onChange={setConfigFilter}
              style={{ width: 160 }}
              optionList={[
                { value: 'all', label: t('全部 ({{count}})', { count: prices.length }) },
                { value: 'configured', label: t('已配置 ({{count}})', { count: configuredCount }) },
                { value: 'unconfigured', label: t('未配置 ({{count}})', { count: unconfiguredCount }) },
              ]}
            />
            <Button icon={<IconPlus />} onClick={() => setAddModelModalVisible(true)}>
              {t('添加模型')}
            </Button>
            <Space>
              <Text>{t('特殊价格显示')}</Text>
              <Switch
                checked={showSpecialPriceColumns}
                onChange={setShowSpecialPriceColumns}
                size="small"
              />
            </Space>
          </Space>
          <Table
            columns={columns}
            dataSource={filteredPrices}
            rowKey="model"
            loading={loading}
            rowSelection={rowSelection}
            pagination={{
              pageSize: 20,
              showSizeChanger: true,
              pageSizeOpts: [10, 20, 50, 100],
            }}
            scroll={{ x: showSpecialPriceColumns ? 1300 : 1000 }}
            empty={<Empty description={t('该分组暂无特殊定价配置')} />}
          />
        </>
      ) : groups.length === 0 && !loading ? (
        <div style={{
          textAlign: 'center',
          padding: '80px 20px',
          backgroundColor: 'var(--semi-color-fill-0)',
          borderRadius: 8
        }}>
          <IconInbox size="extra-large" style={{ color: 'var(--semi-color-text-2)', marginBottom: 16 }} />
          <Title heading={5}>{t('暂无定价分组')}</Title>
          <Text type="tertiary" style={{ display: 'block', marginBottom: 24 }}>
            {t('当前没有任何计费分组，系统将无法为不同级别的用户应用差异化定价')}
          </Text>
          <Button
            type="primary"
            icon={<IconPlus />}
            onClick={() => setCreateGroupModalVisible(true)}
          >
            {t('创建分组')}
          </Button>
        </div>
      ) : (
        <Empty description={t('请选择或创建一个定价分组')} />
      )}

      <CreateGroupModal
        visible={createGroupModalVisible}
        onOk={handleCreateGroup}
        onCancel={() => setCreateGroupModalVisible(false)}
        loading={loading}
      />

      <AddModelModal
        visible={addModelModalVisible}
        onOk={handleAddModel}
        onCancel={() => setAddModelModalVisible(false)}
        loading={loading}
      />

      <SyncFromGroupModal
        visible={syncModalVisible}
        onOk={handleSyncFromGroup}
        onCancel={() => setSyncModalVisible(false)}
        groups={groups}
        selectedGroup={selectedGroup}
        loading={loading}
      />

      <BatchMultiplierModal
        visible={multiplierModalVisible}
        onOk={handleApplyMultiplier}
        onCancel={() => setMultiplierModalVisible(false)}
        groups={groups}
        selectedGroup={selectedGroup}
        selectedCount={selectedRowKeys.length}
        loading={loading}
      />

      {/* 切换分组确认弹窗 */}
      <Modal
        title={t('未保存的更改')}
        visible={!!pendingGroupSwitch}
        onOk={confirmGroupSwitch}
        onCancel={cancelGroupSwitch}
        okText={t('放弃更改')}
        cancelText={t('继续编辑')}
        okType="danger"
      >
        <Text>
          {t('当前分组有 {{count}} 处未保存的更改，切换分组将丢失这些更改。', { count: unsavedCount })}
        </Text>
      </Modal>

      {/* 悬浮操作条 - 有未保存变更时显示 */}
      <FloatingActionToolbar
        visible={hasUnsavedChanges && !!selectedGroup}
        message={(count) => t('有 {{count}} 处未保存的更改', { count })}
        count={unsavedCount}
        actions={[
          {
            label: t('保存全部'),
            icon: <IconSave />,
            type: 'primary',
            onClick: handleBatchSave,
            loading: saveLoading,
          },
          {
            label: t('放弃更改'),
            icon: <IconRefresh />,
            type: 'tertiary',
            onClick: handleDiscardChanges,
          },
        ]}
      />
    </div>
  );
};

export default GroupPricing;
