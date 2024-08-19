import {useParams, useNavigate} from "react-router-dom";
import {useState} from "react";
import {AgGridReact} from "ag-grid-react";

import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-alpine.css';
import {ColDef, SizeColumnsToFitGridStrategy} from "ag-grid-community";
import AsyncSelect from "react-select";

interface UniqueResponse {
    unique: SelectedMedia[];
    library: { id: string, name: string, websiteId: number, isConsortium: boolean }
};

interface SelectedMedia {
    id: string;
    title: string;
    subtitle: string;
    creators: { name: string, role: string }[];
    languages: string[];
    formats: string[];
    description: string;
    coverUrl: string;
    ownedCount: number,
    availableCount: number,
    holdsCount: number,
    estimatedWaitDays: number
};

interface Library {
    id: string;
    websiteId: number;
    name: string;
    isConsortium: boolean;
}

interface LibraryOption {
    value: number;
    label: string;
}

export default function Unique() {
    // const baseUrl = 'http://localhost:8080/';
    const baseUrl = window.location.origin;
    const navigate = useNavigate();
    const [filteredRowCount, setFilteredRowCount] = useState(0);
    const {libraryId} = useParams();
    const libraryIdInt = parseInt(libraryId || '-1');
    const [libraries, setLibraries] = useState<LibraryOption[]>([]);
    const [uniqueResponse, setUniqueResponse] = useState<UniqueResponse>({
        library: {id: '', name: '', websiteId: 0, isConsortium: false},
        unique: [],
    });
    const columnDefs: ColDef[] = [
        {headerName: 'Book Title', field: 'title', minWidth: 250, sort: 'asc'},
        {
            headerName: 'Creator Names', field: 'creators',
            valueFormatter: (params: any) => {
                return params.value.map((creator: any) => creator.name).join(', ');
            }
        },
        {
            headerName: 'Formats', field: 'formats',
            valueFormatter: (params: any) => {
                return params.value.join(', ');
            }
        },

        {
            headerName: 'Languages', field: 'languages',
            valueFormatter: (params: any) => {
                return params.value.join(', ');
            }
        },
        {headerName: 'Owned', field: 'ownedCount', width: 110},
        {headerName: 'Available', field: 'availableCount', sort: 'desc', width: 140},
        {headerName: 'Holds', field: 'holdsCount', width: 110},
        {headerName: 'Estimated Wait Days', field: 'estimatedWaitDays', width: 190},
        {
            headerName: 'Open In Libby',
            field: 'library.id',
            cellRenderer: (params: any) => {
                if (uniqueResponse.unique.length > 0) {
                    return (
                        <a href={`https://libbyapp.com/library/${uniqueResponse.library.id}/generated-36532/page-1/${params.data.id}`}
                           style={{cursor: 'pointer'}}>
                            open in library
                        </a>
                    );
                } else {
                    return null;
                }
            }
        },
    ];


    const libraryOptions = () => {
        let url = new URL('/api/libraries', baseUrl);
        return fetch(url, {
            method: 'GET',
        })
            .then((response) => response.json())
            .then((data) => {
                data.libraries.sort((a: Library, b: Library) => a.name.localeCompare(b.name));
                setLibraries(data.libraries.map((library: Library) => {
                    return {value: library.websiteId, label: library.name};
                }));
            })
            .catch((error) => {
                console.error('Error:', error);
            });
    };

    if (libraries.length === 0) {
        libraryOptions();
    }

    const selectLibraries = (libraryId: number) => {
        let url = new URL('/api/unique', baseUrl);
        let params: any = {websiteId: libraryId};
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));
        // Fetch the availability data
        fetch(url, {
            method: 'GET',
        })
            .then((response) => response.json())
            .then((data) => {
                setUniqueResponse(data);
            })
            .catch((error) => {
                console.error('Error:', error);
            });
    };

    const selectLibrary = (selectedOption: any) => {
        navigate('/unique/' + selectedOption.id);
        selectLibraries(selectedOption.id);
    }

    if (libraryIdInt != -1 && uniqueResponse.library.id === '') {
        selectLibraries(libraryIdInt);
    }

    const autoSizeStrategy: SizeColumnsToFitGridStrategy = {
        type: 'fitGridWidth',
    };

    return (
        <div>
            <h2>Library Unique Media</h2>
            <p>Search for media items that are unique to this library in all of libby's digital libraries.</p>
            <div>
                {libraries.length && (
                    <div>
                        <div style={{display: 'inline'}}>
                            <AsyncSelect
                                placeholder={"Select library"}
                                className={"react-select-container"}
                                classNamePrefix={"react-select"}
                                defaultValue={libraries.filter((option: any) => option.value === libraryIdInt)[0]}
                                options={libraries}
                                onChange={(event) => selectLibrary({id: event ? event.value : -1})}>
                            </AsyncSelect>
                        </div>
                    </div>
                )}
            </div>
            <div>
                {uniqueResponse.unique.length && (
                    <div style={{marginTop: 25}}>
                        <div>{filteredRowCount} shown out of total {uniqueResponse.unique.length}</div>
                        <div className="ag-theme-alpine-auto-dark" style={{height: 600, marginTop: 10}}>
                            <AgGridReact
                                columnDefs={columnDefs}
                                rowData={uniqueResponse.unique}
                                defaultColDef={{
                                    sortable: true,
                                    filter: true,
                                    resizable: true
                                }}
                                autoSizeStrategy={autoSizeStrategy}
                                onFilterChanged={(params) => {
                                    setFilteredRowCount(params.api.getDisplayedRowCount());
                                }}
                                onGridReady={(params) => {
                                    setFilteredRowCount(params.api.getDisplayedRowCount());
                                }}
                                onRowDataUpdated={(params) => {
                                    setFilteredRowCount(params.api.getDisplayedRowCount());
                                }}
                            />
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}