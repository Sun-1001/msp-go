/**
 * 用户增长趋势图表组件
 *
 * 使用 ECharts 按需导入展示用户增长趋势
 */

import React, { useMemo } from 'react';
import ReactEChartsCore from 'echarts-for-react/lib/core';
import * as echarts from 'echarts/core';
import { LineChart } from 'echarts/charts';
import {
  GridComponent,
  TooltipComponent,
  LegendComponent,
} from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import type { UserGrowthDataPoint } from '@/modules/admin/api';

echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer]);

interface UserGrowthChartProps {
  /** 增长数据点列表 */
  data: UserGrowthDataPoint[];
  /** 图表高度 */
  height?: number;
  /** 是否显示加载状态 */
  loading?: boolean;
}

/**
 * 用户增长趋势图表
 */
export const UserGrowthChart: React.FC<UserGrowthChartProps> = ({
  data,
  height = 300,
  loading = false,
}) => {
  const option = useMemo(() => {
    // 格式化日期显示
    const formatDate = (dateStr: string) => {
      const date = new Date(dateStr);
      return `${date.getMonth() + 1}/${date.getDate()}`;
    };

    return {
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(255, 255, 255, 0.95)',
        borderColor: '#e5e7eb',
        borderWidth: 1,
        textStyle: {
          color: '#374151',
        },
        formatter: (params: Array<{ seriesName: string; value: number; axisValue: string; color: string }>) => {
          const date = params[0]?.axisValue || '';
          let html = `<div style="font-weight: 600; margin-bottom: 8px;">${date}</div>`;
          params.forEach((param) => {
            html += `
              <div style="display: flex; align-items: center; margin: 4px 0;">
                <span style="display: inline-block; width: 10px; height: 10px; border-radius: 50%; background: ${param.color}; margin-right: 8px;"></span>
                <span style="flex: 1;">${param.seriesName}</span>
                <span style="font-weight: 600; margin-left: 16px;">${param.value.toLocaleString()}</span>
              </div>
            `;
          });
          return html;
        },
      },
      legend: {
        data: ['总用户', '学生', '教师'],
        bottom: 0,
        textStyle: {
          color: '#6b7280',
        },
        itemWidth: 20,
        itemHeight: 10,
        itemGap: 20,
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '15%',
        top: '10%',
        containLabel: true,
      },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: data.map((d) => d.date),
        axisLabel: {
          color: '#9ca3af',
          formatter: formatDate,
          interval: Math.floor(data.length / 7),
        },
        axisLine: {
          lineStyle: {
            color: '#e5e7eb',
          },
        },
        axisTick: {
          show: false,
        },
      },
      yAxis: {
        type: 'value',
        axisLabel: {
          color: '#9ca3af',
          formatter: (value: number) => {
            if (value >= 1000) {
              return `${(value / 1000).toFixed(1)}k`;
            }
            return value.toString();
          },
        },
        axisLine: {
          show: false,
        },
        axisTick: {
          show: false,
        },
        splitLine: {
          lineStyle: {
            color: '#f3f4f6',
            type: 'dashed',
          },
        },
      },
      series: [
        {
          name: '总用户',
          type: 'line',
          smooth: true,
          symbol: 'circle',
          symbolSize: 6,
          showSymbol: false,
          data: data.map((d) => d.total),
          lineStyle: {
            width: 3,
            color: '#6366f1',
          },
          itemStyle: {
            color: '#6366f1',
          },
          areaStyle: {
            color: {
              type: 'linear',
              x: 0,
              y: 0,
              x2: 0,
              y2: 1,
              colorStops: [
                { offset: 0, color: 'rgba(99, 102, 241, 0.2)' },
                { offset: 1, color: 'rgba(99, 102, 241, 0.02)' },
              ],
            },
          },
        },
        {
          name: '学生',
          type: 'line',
          smooth: true,
          symbol: 'circle',
          symbolSize: 6,
          showSymbol: false,
          data: data.map((d) => d.students),
          lineStyle: {
            width: 2,
            color: '#10b981',
          },
          itemStyle: {
            color: '#10b981',
          },
        },
        {
          name: '教师',
          type: 'line',
          smooth: true,
          symbol: 'circle',
          symbolSize: 6,
          showSymbol: false,
          data: data.map((d) => d.teachers),
          lineStyle: {
            width: 2,
            color: '#f59e0b',
          },
          itemStyle: {
            color: '#f59e0b',
          },
        },
      ],
    };
  }, [data]);

  return (
    <ReactEChartsCore
      echarts={echarts}
      option={option}
      style={{ height }}
      showLoading={loading}
      loadingOption={{
        text: '加载中...',
        color: '#6366f1',
        textColor: '#6b7280',
        maskColor: 'rgba(255, 255, 255, 0.8)',
      }}
    />
  );
};

export default UserGrowthChart;
