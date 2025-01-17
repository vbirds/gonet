package main

import (
	"bytes"
	"fmt"
	"github.com/tealeg/xlsx"
	"gonet/base"
	"os"
	"strings"
)

func OpenExceCsv(filename string){
	xlFile, err := xlsx.OpenFile(filename)
	if err != nil{
		fmt.Println("open [%s] error", filename)
		return
	}

	dataTypes := []int{}
	dataColLen := 0//结束列数
	stream := bytes.NewBuffer([]byte{})
	filenames := strings.Split(filename, ".")
	enumKVMap := make(map[int] map[string] int) //列 key val
	enumKMap := map[int] []string{}//列名对应key
	dataNames := []string{}

	for page, sheet := range xlFile.Sheets{
		if page != 0{
			//other sheet
			continue
			/*for _, row := range sheet.Rows {
				//列不统一
				for j := 0; j < sheet.MaxCol; j ++{
					if j < len(row.Cells){
						stream.WriteString(row.Cells[j].Value)
					}else{
						stream.WriteString("")
					}
				}
			}
			continue*/
		}

		//检查行列
		func(){
			if sheet.MaxRow != len(sheet.Rows){
				fmt.Printf("data [%s] 行数不统一", filename,  )
				return
			}
			for i, row := range sheet.Rows {
				if sheet.MaxCol != len(row.Cells){
					fmt.Printf("data [%s] 列数不统一,第 [%d] 行", filename,  i)
					return
				}
			}
		}()

		for i, row := range sheet.Rows {
			for j, cell := range row.Cells {
				if i == COL_NAME {
					continue
				}else if i == COL_CLIENT_NAME {
					colName := cell.String()
					dataNames = append(dataNames, colName)
					if colName != "" && colName != "0"{
						dataColLen = j
					}
					//最后一次写入
					if j == len(row.Cells) - 1{
						for i1, v := range dataNames{
							if v != "" && v != "0"{
								stream.WriteString(fmt.Sprintf("%s",v))
								if i1 != dataColLen{
									stream.WriteString(",")
								}else if i1 == dataColLen {
									stream.WriteString("\n")
								}
							}
						}
					}
					continue
				} else if i == COL_VSTO {
					if cell.String() == ""{
						continue
					}

					enumNames := strings.Split(cell.String(), "\n")
					for _, v1 := range enumNames{
						enumKMap[j] = append(enumKMap[j], v1)
					}
					continue
				} else if i == COL_TYPE{

					coltype := strings.TrimSpace(strings.ToLower(cell.String()))
					if coltype == "enum"{
						num := 0
						enumKVMap[j] = make(map[string] int)
						for _, v1 := range enumKMap[j]{
							slot := strings.Split(string(v1), "=")
							if len(slot) == 2{
								num = base.Int(slot[1])
								v1 = slot[0]
							}

							enumKVMap[j][v1] = num
							num++
						}
					}
					switch coltype {
					case "string":
						dataTypes = append(dataTypes, base.DType_String)
					case "enum":
						dataTypes = append(dataTypes, base.DType_Enum)
					case "int8":
						dataTypes = append(dataTypes, base.DType_S8)
					case "int16":
						dataTypes = append(dataTypes, base.DType_S16)
					case "int":
						dataTypes = append(dataTypes, base.DType_S32)
					case "float":
						dataTypes = append(dataTypes, base.DType_F32)
					case "float64":
						dataTypes = append(dataTypes, base.DType_F64)
					case "int64":
						dataTypes = append(dataTypes, base.DType_S64)

					case "[]string":
						dataTypes = append(dataTypes, base.DType_StringArray)
					case "[]int8":
						dataTypes = append(dataTypes, base.DType_S8Array)
					case "[]int16":
						dataTypes = append(dataTypes, base.DType_S16Array)
					case "[]int":
						dataTypes = append(dataTypes, base.DType_S32Array)
					case "[]float":
						dataTypes = append(dataTypes, base.DType_F32Array)
					case "[]float64":
						dataTypes = append(dataTypes, base.DType_F64Array)
					case "[]int64":
						dataTypes = append(dataTypes, base.DType_S64Array)
					default:
						fmt.Printf("data [%s] [%s] col[%d] type not support in[string, enum, int8, int16, int32, float32, float64, []string, []int8, []int16, []int32, []float32, []float64]", filename, coltype, j )
						return
					}
					continue
				}

				//过滤掉不是客户端的数据
				if dataNames[j] == "" || dataNames[j] == "0"{
					continue
				}

				switch dataTypes[j]{
				case base.DType_String:
					stream.WriteString(cell.Value)
				case base.DType_Enum:
					val, bEx := enumKVMap[j][strings.ToLower(cell.Value)]
					if bEx{
						stream.WriteString(fmt.Sprintf("%d", val))
					}else{
						stream.WriteString(fmt.Sprintf("%d", 0))
					}
				case base.DType_S8:
					stream.WriteString(fmt.Sprintf("%d", base.Int(cell.Value)))
				case base.DType_S16:
					stream.WriteString(fmt.Sprintf("%d", base.Int(cell.Value)))
				case base.DType_S32:
					stream.WriteString(fmt.Sprintf("%d", base.Int(cell.Value)))
				case base.DType_F32:
					stream.WriteString(fmt.Sprintf("%f", base.Float32(cell.Value)))
				case base.DType_F64:
					stream.WriteString(fmt.Sprintf("%f", base.Float64(cell.Value)))
				case  base.DType_S64:
					stream.WriteString(fmt.Sprintf("%d", base.Int64(cell.Value)))
				case base.DType_StringArray, base.DType_S8Array, base.DType_S16Array, base.DType_S32Array,
					base.DType_F32Array, base.DType_F64Array, base.DType_S64Array:
					stream.WriteString(cell.Value)
				}

				if j != dataColLen{
					stream.WriteString(",")
				}else if j == dataColLen{
					stream.WriteString("\n")
				}
			}
		}
	}

	//文件没有可导出
	if dataColLen == 0{
		return
	}
	/*stream.WriteString(fmt.Sprintf("%s %s%s","local", filenames[0],"DataName = {\n" ))
	for _, v := range dataNames{
		stream.WriteString(fmt.Sprintf("\t%s,\n", v))
	}
	stream.WriteString("}\n")
	stream.WriteString(fmt.Sprintf("%s %s%s\n","return", filenames[0],"DataName" ))*/
	//other sheet
	file, err := os.Create(filenames[0] + ".csv")
	if err == nil{
		file.Write(stream.Bytes())
		file.Close()
	}
}

