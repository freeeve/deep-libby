import {useParams} from "react-router-dom";
import {useState} from "react";
import {AgGridReact} from "ag-grid-react";

import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-alpine.css';
import {ColDef, SizeColumnsToFitGridStrategy} from "ag-grid-community";

interface SelectedMedia {
    id: string;
    title: string;
    subtitle: string;
    creators: { name: string, role: string }[];
    languages: string[];
    formats: string[];
    description: string;
    coverUrl: string;
    seriesName: string;
    seriesReadOrder: number;
    libraryCount: number;
    availability: {
        library: { id: string, name: string, websiteId: number },
        ownedCount: number,
        availableCount: number,
        holdsCount: number,
        estimatedWaitDays: number
    }[];
}

export default function Availability() {
    let baseUrl = window.location.origin;
    if (baseUrl === 'http://localhost:5173') {
        baseUrl = 'http://localhost:8080';
    }
    const {mediaId} = useParams();
    console.log(mediaId);
    const [selectedMedia, setSelectedMedia] = useState<SelectedMedia | null>(null);
    console.log("mediaId", mediaId);
    const [favorites, setFavorites] = useState<number[]>([]);

    function memoFavorites() {
        if (favorites.length !== 0) {
            return favorites;
        }
        let favs: string = localStorage.getItem('favorites') || '[]';
        if (favs === '[]') {
            setFavorites([]);
            return [];
        } else {
            setFavorites(JSON.parse(favs));
            return JSON.parse(favs);
        }
    }

    const columnDefs: ColDef[] = [
        {headerName: 'Library Name', field: 'library.name', minWidth: 400},
        {
            headerName: 'Fav.', field: 'library.favorite', sort: 'desc', width: 130,
        },
        {headerName: 'Owned', field: 'ownedCount', width: 110},
        {headerName: 'Available', field: 'availableCount', sort: 'desc', width: 140},
        {headerName: 'Holds', field: 'holdsCount', width: 110},
        {headerName: 'Estimated Wait Days', field: 'estimatedWaitDays', sort: 'asc', width: 190},
        {
            headerName: 'Open In Libby',
            field: 'library.id',
            cellRenderer: (params: any) => {
                if (selectedMedia !== null) {
                    return (
                        <a href={`https://libbyapp.com/library/${params.value}/generated-36532/page-1/${selectedMedia.id}`}
                           style={{cursor: 'pointer'}}>
                            open in this library
                        </a>
                    );
                } else {
                    return null; // or some default JSX
                }
            }
        }
    ];

    const clickMedia = (selectedOption: any) => {
        let url = new URL('/api/availability', baseUrl);
        let params: any = {id: selectedOption.id};
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));
        // Fetch the availability data
        fetch(url, {
            method: 'GET',
        })
            .then((response) => response.json())
            .then((data) => {
                // Update the state with the selected book's details and availability data
                data.availability.forEach((item: any) => {
                    item.library.favorite = memoFavorites().includes(item.library.websiteId);
                });
                setSelectedMedia(data);
            })
            .catch((error) => {
                console.error('Error:', error);
            });
    };
    if (selectedMedia === null) {
        clickMedia({id: mediaId})
    }

    const autoSizeStrategy: SizeColumnsToFitGridStrategy = {
        type: 'fitGridWidth',
    };

    return (
        selectedMedia && (
            <div>
                <h2>Media Availability</h2>
                <div style={{display: 'flex', justifyContent: 'space-between'}}>
                    <div>
                        <div style={{textAlign: 'left'}}>
                            <div><strong>Title:</strong> <strong>{selectedMedia.title}</strong></div>
                            {selectedMedia.seriesName !== "" && <div><strong>Series:</strong>
                                <strong>#{selectedMedia.seriesReadOrder} in {selectedMedia.seriesName}</strong></div>}
                            <div>
                                <strong>Creators:</strong> {selectedMedia.creators.map((author) => author.name + ' (' + author.role + ')').join(', ')}
                            </div>
                            <div><strong>Languages:</strong> {selectedMedia.languages.join(', ')}</div>
                            <div><strong>Formats:</strong> {selectedMedia.formats.join(', ')}</div>
                            {selectedMedia.subtitle != "" && (
                                <div><strong>Subtitle:</strong> {selectedMedia.subtitle}</div>)}
                            <div className={'dangerousHTML'}>
                                <div><strong>Description:</strong></div>
                                <div dangerouslySetInnerHTML={{__html: selectedMedia.description}}></div>
                            </div>
                            <div><a href={'https://www.overdrive.com/media/' + selectedMedia.id}>open in overdrive</a>
                            </div>
                        </div>
                    </div>
                    <div style={{textAlign: 'left'}}>
                        <span style={{verticalAlign: 'top', marginRight: 5}}>owned by {selectedMedia.libraryCount} libraries</span>
                        <img src={selectedMedia.coverUrl}
                             alt={selectedMedia.title}
                             width={0} height={0}
                             sizes="100vw"
                             style={{width: 'auto', height: '100px'}} // optional
                        />
                    </div>
                </div>
                <div className="ag-theme-alpine-auto-dark" style={{height: 600, marginTop: 25}}>
                    <AgGridReact
                        columnDefs={columnDefs}
                        rowData={selectedMedia.availability}
                        defaultColDef={{
                            sortable: true,
                            filter: true,
                            resizable: true
                        }}
                        autoSizeStrategy={autoSizeStrategy}
                    />
                </div>
            </div>
        ));
}